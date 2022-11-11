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

package cmd

import (
	"github.com/DIMO-Network/edge-identity/p11"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/cobra"
)

// listCmd represents the list command
var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify Signature",
	Run: func(cmd *cobra.Command, args []string) {
		doVerify(cmd)
	},
}
var signature string

func init() {
	rootCmd.AddCommand(verifyCmd)

	verifyCmd.Flags().StringVar(&label, "label", "", "Use token with this label")
	verifyCmd.Flags().StringVar(&signature, "signature", "", "Signature to verify")
	verifyCmd.Flags().StringVar(&keyid, "keyid", "", "Use token with this keyid")
	verifyCmd.Flags().StringVar(&message, "message", "", "Original message")
	verifyCmd.Flags().StringVar(&hash, "hash", "", "Original hash")

	verifyCmd.MarkFlagRequired("label")
	verifyCmd.MarkFlagRequired("signature")
	verifyCmd.MarkFlagsMutuallyExclusive("message", "hash")
}

func doVerify(cmd *cobra.Command) {

	var err error
	var labelToUse string
	if cmd.Flags().Changed("label") {
		labelToUse = label
	}

	var keyIdToUse string
	if cmd.Flags().Changed("keyid") {
		keyIdToUse = keyid
	}

	var signatureTouse *string
	if cmd.Flags().Changed("signature") {
		signatureTouse = &signature
	}

	var hashToVerify []byte
	if cmd.Flags().Changed("message") {
		hashToVerify = crypto.Keccak256([]byte(message))
	}

	if cmd.Flags().Changed("hash") {
		hashToVerify, err = hexutil.Decode(hash)
		handleError(err)
	}
	p11Token, err := p11.NewToken(p11Lib, p11TokenLabel, getPIN(cmd))
	handleError(err)
	defer p11Token.Finalise()

	sig, err := hexutil.Decode(*signatureTouse)
	handleError(err)

	err = p11Token.Verify(labelToUse, keyIdToUse, hashToVerify, sig)
	handleError(err)
}
