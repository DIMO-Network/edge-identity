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
	"log"

	"github.com/DIMO-Network/edge-identity/p11"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/cobra"
)

// listCmd represents the list command
var signCmd = &cobra.Command{
	Use:   "sign",
	Short: "Sign payload message",
	Run: func(cmd *cobra.Command, args []string) {
		doSign(cmd)
	},
}

func init() {
	rootCmd.AddCommand(signCmd)

	signCmd.Flags().StringVar(&label, "label", "", "Use token with this label")
	signCmd.Flags().StringVar(&keyid, "keyid", "", "Use token with this keyid")
	signCmd.Flags().StringVar(&hash, "hash", "", "Hash to sign")
	signCmd.Flags().StringVar(&message, "message", "", "Message to sign")

	signCmd.MarkFlagRequired("label")
	signCmd.MarkFlagsMutuallyExclusive("message", "hash")
}

func doSign(cmd *cobra.Command) {
	var err error
	var labelToUse string
	if cmd.Flags().Changed("label") {
		labelToUse = label
	}

	var keyIdToUse string
	if cmd.Flags().Changed("keyid") {
		keyIdToUse = keyid
	}
	var hashToSign []byte
	if cmd.Flags().Changed("message") {
		hashToSign = crypto.Keccak256([]byte(message))
	}

	if cmd.Flags().Changed("hash") {
		hashToSign, err = hexutil.Decode(hash)
		handleError(err)
	}
	p11Token, err := p11.NewToken(p11Lib, p11TokenLabel, getPIN(cmd))
	handleError(err)
	defer p11Token.Finalise()

	result, err := p11Token.Sign(labelToUse, keyIdToUse, hashToSign)
	handleError(err)
	log.Printf("Signature %s", hexutil.Encode(result))
}
