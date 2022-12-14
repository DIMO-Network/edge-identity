// Copyright 2018 Thales UK Limited
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated
// documentation files (the "Software"), to deal in the Software without restriction, including without limitation the
// rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to
// permit persons to whom the Software is furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all copies or substantial portions of the
// Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE
// WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
// COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR
// OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package p11

import (
	"crypto/ecdsa"
	"encoding/asn1"
	"fmt"
	"log"
	"math/big"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/miekg/pkcs11"
	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
)

var secp256k1N = crypto.S256().Params().N
var secp256k1HalfN = new(big.Int).Div(secp256k1N, big.NewInt(2))

// TokenCtx contains the functions we use from github.com/miekg/pkcs11.
type TokenCtx interface {
	CloseSession(sh pkcs11.SessionHandle) error
	CreateObject(sh pkcs11.SessionHandle, temp []*pkcs11.Attribute) (pkcs11.ObjectHandle, error)
	Destroy()
	DestroyObject(sh pkcs11.SessionHandle, oh pkcs11.ObjectHandle) error
	Encrypt(sh pkcs11.SessionHandle, message []byte) ([]byte, error)
	EncryptInit(sh pkcs11.SessionHandle, m []*pkcs11.Mechanism, o pkcs11.ObjectHandle) error
	Finalize() error
	FindObjects(sh pkcs11.SessionHandle, max int) ([]pkcs11.ObjectHandle, bool, error)
	FindObjectsFinal(sh pkcs11.SessionHandle) error
	FindObjectsInit(sh pkcs11.SessionHandle, temp []*pkcs11.Attribute) error
	GenerateKey(sh pkcs11.SessionHandle, mech []*pkcs11.Mechanism, temp []*pkcs11.Attribute) (pkcs11.ObjectHandle, error)
	GenerateKeyPair(sh pkcs11.SessionHandle, mech []*pkcs11.Mechanism, public, private []*pkcs11.Attribute) (pkcs11.ObjectHandle, pkcs11.ObjectHandle, error)
	GetAttributeValue(sh pkcs11.SessionHandle, o pkcs11.ObjectHandle, a []*pkcs11.Attribute) ([]*pkcs11.Attribute, error)
	GetSlotList(tokenPresent bool) ([]uint, error)
	GetTokenInfo(slotID uint) (pkcs11.TokenInfo, error)
	Initialize() error
	SignInit(sh pkcs11.SessionHandle, m []*pkcs11.Mechanism, o pkcs11.ObjectHandle) error
	Sign(sh pkcs11.SessionHandle, message []byte) ([]byte, error)
	Login(sh pkcs11.SessionHandle, userType uint, pin string) error
	OpenSession(slotID uint, flags uint) (pkcs11.SessionHandle, error)
	GetMechanismList(slotID uint) ([]*pkcs11.Mechanism, error)
	GetMechanismInfo(slotID uint, m []*pkcs11.Mechanism) (pkcs11.MechanismInfo, error)
}

// Token provides a high level interface to a P11 token.
type Token interface {
	// Checksum calculates a checksum value for an AES key. A block of zeroes is encrypted in CBC-mode with a zero IV.
	Checksum(keyLabel string) ([]byte, error)

	// ImportKey imports an AES key and applies a label.
	ImportKey(keyBytes []byte, label string) error

	// DeleteAllExcept deletes all keys on the token except those with a label specified.
	DeleteAllExcept(keyLabels []string) error

	// PrintObjects prints all objects in the token if label is nil, otherwise it prints only the objects with that
	// label
	PrintObjects(label *string) error

	// GenerateKey creates a new RSA or AES or EC key of the given size in the token
	GenerateKeyPair(label string, keyid string, algorithm string, keytype string, keysize int) error

	// GenerateKey creates a new RSA or AES key of the given size in the token
	GetPublicKey(label string, keyid string) (publicKey *ecdsa.PublicKey, keyBytes []byte, err error)

	// Sign returns a signature using the in-built curve
	Sign(label string, keyid string, hash []byte) (signature []byte, err error)

	// Verify checks the provided hash against the provisioned address
	Verify(label string, keyid string, hash []byte, signature []byte) (err error)

	// PrintMechanisms prints mechanism info for all supported mechanisms.
	PrintMechanisms() error

	// Finalise closes the library and unloads it.
	Finalise() error
}

type p11Token struct {
	ctx     TokenCtx
	session pkcs11.SessionHandle
	slot    uint
}

func (p *p11Token) DeleteAllExcept(keyLabels []string) error {
	objects, err := p.findAllMatching(nil)
	if err != nil {
		return err
	}

	template := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_LABEL, nil),
	}

	for _, o := range objects {
		labelExists := true

		template, err = p.ctx.GetAttributeValue(p.session, o, template)
		if err != nil {
			if p11error, ok := err.(pkcs11.Error); ok {
				if p11error == pkcs11.CKR_ATTRIBUTE_TYPE_INVALID {
					// There is no label associated with this key
					log.Println("Failed to get label for key, will delete anyway")
					labelExists = false
				} else {
					return errors.WithMessage(err, "failed to get label")
				}
			} else {
				return errors.WithMessage(err, "failed to get label")
			}

		}

		keep := false

		if labelExists {
			for _, l := range keyLabels {
				if l == string(template[0].Value) {
					keep = true
					break
				}
			}
		}

		if !keep {
			if labelExists {
				log.Printf("Deleting key with label '%s'", string(template[0].Value))
			}

			err = p.ctx.DestroyObject(p.session, o)
			if err != nil {
				return errors.WithMessage(err, "failed to destroy object")
			}
		}
	}

	return nil
}

func (p *p11Token) Finalise() error {
	err := p.ctx.Finalize()
	if err != nil {
		return errors.WithMessage(err, "failed to finalize library")
	}

	p.ctx.Destroy()
	return nil
}

// NewToken connects to a PKCS#11 token and creates a logged in, ready-to-use interface. Call Finalize() on the
// return object when finished.
func NewToken(lib, tokenLabel, pin string) (Token, error) {
	ctx := pkcs11.New(lib)
	if ctx == nil {
		return nil, errors.Errorf("failed to load library %s", lib)
	}

	return newP11Token(ctx, tokenLabel, pin)
}

func newP11Token(ctx TokenCtx, tokenLabel, pin string) (Token, error) {
	err := ctx.Initialize()
	if err != nil {
		return nil, err
	}

	session, slot, err := openUserSession(ctx, tokenLabel, pin)
	return &p11Token{
		ctx:     ctx,
		session: session,
		slot:    slot,
	}, err
}

func (p *p11Token) Checksum(keyLabel string) (checksum []byte, err error) {
	var obj pkcs11.ObjectHandle
	obj, err = p.findKeyByLabel(keyLabel)
	if err != nil {
		return
	}

	mech := []*pkcs11.Mechanism{pkcs11.NewMechanism(pkcs11.CKM_AES_CBC, make([]byte, 16))}

	err = p.ctx.EncryptInit(p.session, mech, obj)
	if err != nil {
		return
	}

	checksum, err = p.ctx.Encrypt(p.session, make([]byte, 16))
	return
}

func (p *p11Token) findKeyByLabel(label string) (obj pkcs11.ObjectHandle, err error) {
	template := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_LABEL, label),
	}

	err = p.ctx.FindObjectsInit(p.session, template)
	if err != nil {
		return
	}

	var objects []pkcs11.ObjectHandle
	objects, _, err = p.ctx.FindObjects(p.session, 1)

	if len(objects) != 1 {
		err = errors.Errorf("no key with label '%s'", label)
		return
	}

	obj = objects[0]

	err = p.ctx.FindObjectsFinal(p.session)
	return
}

func (p *p11Token) ImportKey(keyBytes []byte, label string) error {
	template := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_SECRET_KEY),
		pkcs11.NewAttribute(pkcs11.CKA_KEY_TYPE, pkcs11.CKK_AES),
		pkcs11.NewAttribute(pkcs11.CKA_VALUE, keyBytes),
		pkcs11.NewAttribute(pkcs11.CKA_UNWRAP, true),
		pkcs11.NewAttribute(pkcs11.CKA_WRAP, true),
		pkcs11.NewAttribute(pkcs11.CKA_SENSITIVE, true),
		pkcs11.NewAttribute(pkcs11.CKA_TOKEN, true),
		pkcs11.NewAttribute(pkcs11.CKA_LABEL, label),
	}

	_, err := p.ctx.CreateObject(p.session, template)
	return err
}

// openP11Session loads the P11 library and creates a logged in session
func openUserSession(ctx TokenCtx, tokenLabel, pin string) (session pkcs11.SessionHandle, slot uint, err error) {
	slot, err = findSlotWithToken(ctx, tokenLabel)
	if err != nil {
		return
	}

	session, err = ctx.OpenSession(slot, pkcs11.CKF_SERIAL_SESSION|pkcs11.CKF_RW_SESSION)
	if err != nil {
		return
	}

	err = ctx.Login(session, pkcs11.CKU_USER, pin)
	return
}

// findSlotWithToken returns the (first) slot id containing the specific token. If the token is not found an
// error is returned.
func findSlotWithToken(ctx TokenCtx, label string) (slot uint, err error) {
	var slots []uint
	slots, err = ctx.GetSlotList(true)
	if err != nil {
		return
	}

	for _, slot = range slots {
		var info pkcs11.TokenInfo
		info, err = ctx.GetTokenInfo(slot)
		if err != nil {
			return
		}

		if info.Label == label {
			return
		}
	}

	err = errors.Errorf("cannot find token %s", label)
	return
}

func (p *p11Token) findAllMatching(template []*pkcs11.Attribute) (objects []pkcs11.ObjectHandle, err error) {
	const batchSize = 20

	err = p.ctx.FindObjectsInit(p.session, template)
	if err != nil {
		return
	}

	var res []pkcs11.ObjectHandle
	for {
		// The 'more' return value is broken, don't use
		res, _, err = p.ctx.FindObjects(p.session, batchSize)
		if err != nil {
			err = errors.WithMessage(err, "failed to search")
			return
		}

		if len(res) == 0 {
			//log.Printf("Found %d objects on token", len(objects))
			break
		}

		objects = append(objects, res...)
	}

	err = p.ctx.FindObjectsFinal(p.session)
	return
}

func (p *p11Token) PrintObjects(label *string) error {
	var template []*pkcs11.Attribute
	if label != nil {
		template = append(template, pkcs11.NewAttribute(pkcs11.CKA_LABEL, *label))
	}

	objects, err := p.findAllMatching(template)
	if err != nil {
		return err
	}

	for i, o := range objects {
		err := printObject(p.ctx, p.session, o, i+1)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *p11Token) GetPublicKey(label string, keyid string) (publicKey *ecdsa.PublicKey, keyBytes []byte, err error) {
	var template []*pkcs11.Attribute
	template = append(template, pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_PUBLIC_KEY))
	if label != "" {
		template = append(template, pkcs11.NewAttribute(pkcs11.CKA_LABEL, label))
	}
	if keyid != "" {
		template = append(template, pkcs11.NewAttribute(pkcs11.CKA_ID, keyid))
	}

	objects, err := p.findAllMatching(template)
	if err != nil {
		return nil, nil, err
	}
	if len(objects) > 1 {
		return nil, nil, errors.New("More than 1 matching key found, please specify both label and key id")
	}

	if len(objects) == 0 {
		return nil, nil, errors.New("No matching keys found")
	}

	ecpt := ecPoint(p.ctx, p.session, objects[0])

	pub, err := crypto.UnmarshalPubkey(ecpt)
	if err != nil {
		log.Println(err)
	}

	return pub, ecpt, err
}

func (p *p11Token) Sign(label string, keyid string, hash []byte) (signature []byte, err error) {
	var template []*pkcs11.Attribute
	template = append(template, pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_PRIVATE_KEY))
	if label != "" {
		template = append(template, pkcs11.NewAttribute(pkcs11.CKA_LABEL, label))
	}

	if keyid != "" {
		template = append(template, pkcs11.NewAttribute(pkcs11.CKA_ID, keyid))
	}

	objects, err := p.findAllMatching(template)
	if err != nil {
		return nil, err
	}
	if len(objects) > 1 {
		return nil, errors.New("More than 1 matching key found, please specify both label and key id")
	}

	if len(objects) == 0 {
		return nil, errors.New("No matching keys found")
	}

	err = p.ctx.SignInit(p.session, []*pkcs11.Mechanism{pkcs11.NewMechanism(pkcs11.CKM_ECDSA, nil)}, objects[0])
	if err != nil {
		log.Fatalf("Signing Initiation failed (%s)\n", err.Error())
	}

	// Sign Msg
	sig, err := p.ctx.Sign(p.session, hash)
	if err != nil {
		return nil, err
	}
	// Get Public Key
	_, ecpt, err := p.GetPublicKey(label, keyid)
	if err != nil {
		return nil, err
	}

	sigR, sigS := sig[:32], sig[32:64]

	// Correct S, if necessary, so that it's in the lower half of the group.
	sigSNum := new(big.Int).SetBytes(sigS)
	if sigSNum.Cmp(secp256k1HalfN) > 0 {
		sigS = new(big.Int).Sub(secp256k1N, sigSNum).Bytes()
	}

	// Determine whether V ought to be 0 or 1.
	sigRS := append(fixLen(sigR), fixLen(sigS)...)
	sigRSV := append(sigRS, 0)

	recPub, err := crypto.Ecrecover(hash[:], sigRSV)
	if err != nil {
		return nil, err
	}

	if slices.Equal(recPub, ecpt) {
		sigRSV[64] += 27
		return sigRSV, nil
	}

	sigRSV = append(sigRS, 1)
	recPub, err = crypto.Ecrecover(hash[:], sigRSV)
	if err != nil {
		return nil, err
	}

	if slices.Equal(recPub, ecpt) {
		sigRSV[64] += 27
		return sigRSV, nil
	}

	return nil, errors.New("Could not generate a valid signature")
}

func (p *p11Token) Verify(label string, keyid string, hash []byte, signature []byte) (err error) {
	_, ecpt, err := p.GetPublicKey(label, keyid)
	if err != nil {
		return err
	}
	recPub, err := crypto.Ecrecover(hash[:], signature)
	if err != nil {
		return err
	}

	if slices.Equal(recPub, ecpt) {
		log.Println("Verified successfully")
		return nil
	}
	return errors.New("Not verified")
}

func (p *p11Token) GenerateKeyPair(label string, keyid string, algorithm string, keytype string, keysize int) error {

	validRSASize := []int{1024, 2048, 3072, 4096}
	validAESSize := []int{128, 192, 256}
	validECSize := []int{128, 192, 256}

	switch keytype {
	case "RSA":
		if isValidSize(validRSASize, keysize) {
			return p.GenerateRSAKey(label, keysize)
		} else {
			return errors.Errorf("Invalid RSA key size: %d", keysize)
		}
	case "AES":
		if isValidSize(validAESSize, keysize) {
			return p.GenerateAESKey(label, keysize)
		} else {
			return errors.Errorf("Invalid AES key size: %d", keysize)
		}
	case "EC":
		if isValidSize(validECSize, keysize) && algorithm == "S256" {
			return p.GenerateECKey(label, keyid)
		} else {
			return errors.Errorf("Invalid EC key size: %d", keysize)
		}
	default:
		return errors.Errorf("Invalid key type: %s", keytype)
	}
}

func (p *p11Token) GenerateECKey(label string, keyid string) error {
	var template []*pkcs11.Attribute
	template = append(template, pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_PUBLIC_KEY))
	if label != "" {
		template = append(template, pkcs11.NewAttribute(pkcs11.CKA_LABEL, label))
	}

	objects, err := p.findAllMatching(template)
	if err != nil {
		return err
	}

	if len(objects) > 0 {
		return errors.New("Key with this label already exists")
	}

	marshaledOID, _ := asn1.Marshal(asn1.ObjectIdentifier{1, 3, 132, 0, 10})
	publicKeyTemplate := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_KEY_TYPE, pkcs11.CKK_EC),
		pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_PUBLIC_KEY),
		pkcs11.NewAttribute(pkcs11.CKA_TOKEN, true),
		pkcs11.NewAttribute(pkcs11.CKA_VERIFY, true),
		pkcs11.NewAttribute(pkcs11.CKA_EC_PARAMS, marshaledOID),
		pkcs11.NewAttribute(pkcs11.CKA_LABEL, label),
	}

	privateKeyTemplate := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_KEY_TYPE, pkcs11.CKK_EC),
		pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_PRIVATE_KEY),
		pkcs11.NewAttribute(pkcs11.CKA_TOKEN, true),
		pkcs11.NewAttribute(pkcs11.CKA_PRIVATE, true),
		pkcs11.NewAttribute(pkcs11.CKA_SIGN, true),

		pkcs11.NewAttribute(pkcs11.CKA_LABEL, label),

		pkcs11.NewAttribute(pkcs11.CKA_EXTRACTABLE, false),
		pkcs11.NewAttribute(pkcs11.CKA_SENSITIVE, true),
		// CKA_MODULUS
		// CKA_MODULUS_BITS
		// CKA_PUBLIC_EXPONENT
		// CKA_PRIVATE_EXPONENT
		// CKA_PRIME_1
		// CKA_PRIME_2
	}
	if keyid != "" {
		privateKeyTemplate = append(privateKeyTemplate, pkcs11.NewAttribute(pkcs11.CKA_ID, keyid))
		publicKeyTemplate = append(publicKeyTemplate, pkcs11.NewAttribute(pkcs11.CKA_ID, keyid))
	} else {
		privateKeyTemplate = append(privateKeyTemplate, pkcs11.NewAttribute(pkcs11.CKA_ID, label))
		publicKeyTemplate = append(publicKeyTemplate, pkcs11.NewAttribute(pkcs11.CKA_ID, label))
	}

	_, _, err = p.ctx.GenerateKeyPair(p.session,
		[]*pkcs11.Mechanism{pkcs11.NewMechanism(pkcs11.CKM_EC_KEY_PAIR_GEN, nil)},
		publicKeyTemplate, privateKeyTemplate)

	if err != nil {
		return err
	}

	log.Printf("Keypair \"%s\" generated on token", label)

	return nil
}

func (p *p11Token) GenerateAESKey(label string, keysize int) error {

	privateKeyTemplate := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_TOKEN, true),
		pkcs11.NewAttribute(pkcs11.CKA_SIGN, true),
		pkcs11.NewAttribute(pkcs11.CKA_LABEL, label),
		pkcs11.NewAttribute(pkcs11.CKA_SENSITIVE, true),
		pkcs11.NewAttribute(pkcs11.CKA_EXTRACTABLE, true),
		pkcs11.NewAttribute(pkcs11.CKA_VALUE_LEN, keysize/8),
	}

	_, err := p.ctx.GenerateKey(p.session,
		[]*pkcs11.Mechanism{pkcs11.NewMechanism(pkcs11.CKM_AES_KEY_GEN, make([]byte, 16))},
		privateKeyTemplate)

	if err != nil {
		return err
	}

	log.Printf("Key \"%s\" generated on token", label)

	return nil
}

func (p *p11Token) GenerateRSAKey(label string, keysize int) error {

	publicKeyTemplate := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_PUBLIC_KEY),
		pkcs11.NewAttribute(pkcs11.CKA_KEY_TYPE, pkcs11.CKK_RSA),
		pkcs11.NewAttribute(pkcs11.CKA_TOKEN, true),
		pkcs11.NewAttribute(pkcs11.CKA_VERIFY, true),
		pkcs11.NewAttribute(pkcs11.CKA_PUBLIC_EXPONENT, []byte{1, 0, 1}),
		pkcs11.NewAttribute(pkcs11.CKA_LABEL, label),
		pkcs11.NewAttribute(pkcs11.CKA_MODULUS_BITS, keysize),
	}

	privateKeyTemplate := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_EXTRACTABLE, true),
		pkcs11.NewAttribute(pkcs11.CKA_TOKEN, true),
		pkcs11.NewAttribute(pkcs11.CKA_SENSITIVE, true),
		pkcs11.NewAttribute(pkcs11.CKA_SIGN, true),
		pkcs11.NewAttribute(pkcs11.CKA_LABEL, label),
	}

	_, _, err := p.ctx.GenerateKeyPair(p.session,
		[]*pkcs11.Mechanism{pkcs11.NewMechanism(pkcs11.CKM_RSA_PKCS_KEY_PAIR_GEN, nil)},
		publicKeyTemplate, privateKeyTemplate)

	if err != nil {
		return err
	}

	log.Printf("Keypair \"%s\" generated on token", label)

	return nil
}

func (p *p11Token) PrintMechanisms() error {
	mechs, err := p.ctx.GetMechanismList(p.slot)
	if err != nil {
		return err
	}

	// Sort alphabetically by name
	sort.Slice(mechs, func(i, j int) bool {
		return strings.Compare(mechToStringAlways(mechs[i].Mechanism), mechToStringAlways(mechs[j].Mechanism)) < 0
	})

	for _, m := range mechs {
		fmt.Println(mechToStringAlways(m.Mechanism))
		info, err := p.ctx.GetMechanismInfo(p.slot, []*pkcs11.Mechanism{m})
		if err != nil {
			return err
		}

		fmt.Printf("  MinKeySize=%d, MaxKeySize=%d\n", info.MinKeySize, info.MaxKeySize)

		possibleFlags := map[string]uint{
			"CKF_HW":                pkcs11.CKF_HW,
			"CKF_ENCRYPT":           pkcs11.CKF_ENCRYPT,
			"CKF_DECRYPT":           pkcs11.CKF_DECRYPT,
			"CKF_DIGEST":            pkcs11.CKF_DIGEST,
			"CKF_SIGN":              pkcs11.CKF_SIGN,
			"CKF_SIGN_RECOVER":      pkcs11.CKF_SIGN_RECOVER,
			"CKF_VERIFY":            pkcs11.CKF_VERIFY,
			"CKF_VERIFY_RECOVER":    pkcs11.CKF_VERIFY_RECOVER,
			"CKF_GENERATE":          pkcs11.CKF_GENERATE,
			"CKF_GENERATE_KEY_PAIR": pkcs11.CKF_GENERATE_KEY_PAIR,
			"CKF_WRAP":              pkcs11.CKF_WRAP,
			"CKF_UNWRAP":            pkcs11.CKF_UNWRAP,
			"CKF_DERIVE":            pkcs11.CKF_DERIVE,
		}

		var flags []string

		for name, value := range possibleFlags {
			if (info.Flags & value) != 0 {
				flags = append(flags, name)
			}
		}
		sort.Strings(flags)

		fmt.Printf("  Flags=%s\n", strings.Join(flags, ", "))
	}

	return nil
}

func isValidSize(sizes []int, in int) bool {
	for _, n := range sizes {
		if in == n {
			return true
		}
	}
	return false
}

func ecPoint(pkcs11lib TokenCtx, session pkcs11.SessionHandle, key pkcs11.ObjectHandle) (ecpt []byte) {
	template := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_EC_POINT, nil),
	}

	attr, err := pkcs11lib.GetAttributeValue(session, key, template)
	if err != nil {
		log.Println("Error getting ec point")
	}

	for _, a := range attr {
		switch {

		case ((len(a.Value) % 2) == 0) && (byte(0x04) == a.Value[0]) && (byte(0x04) == a.Value[len(a.Value)-1]):
			ecpt = a.Value[0 : len(a.Value)-1] // Trim trailing 0x04
		case byte(0x04) == a.Value[0] && byte(0x04) == a.Value[2]:
			ecpt = a.Value[2:len(a.Value)]
		default:
			ecpt = a.Value
		}
	}

	return ecpt
}

func recoverAddress(hash []byte, signature []byte) (addr common.Address, err error) {
	if signature[64] == 27 || signature[64] == 28 {
		signature[64] -= 27
	}
	rawPub, err := crypto.Ecrecover(hash[:], signature)
	if err != nil {
		return
	}

	pub, err := crypto.UnmarshalPubkey(rawPub)
	if err != nil {
		return
	}
	addr = crypto.PubkeyToAddress(*pub)
	return
}

func fixLen(b []byte) []byte {
	i := 0
	for i < len(b) {
		if b[i] != 0 {
			break
		}
		i++
	}

	out := make([]byte, common.HashLength)
	copy(out, b[i:])
	return out
}
