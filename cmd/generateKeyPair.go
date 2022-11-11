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
	"github.com/spf13/cobra"
)

var keytype string
var keysize int
var algorithm string

// generateCmd represents the generate command
var generateKeyPair = &cobra.Command{
	Use:   "generateKeyPair",
	Short: "Generate a new SECP256K1 key pair",
	Run: func(cmd *cobra.Command, args []string) {
		doGenerateKeyPair(cmd)
	},
}

func init() {
	rootCmd.AddCommand(generateKeyPair)

	generateKeyPair.Flags().StringVar(&label, "label", "", "Label for generated key [required]")
	generateKeyPair.Flags().StringVar(&keyid, "keyid", "", "KeyId for generated key [required]")
	generateKeyPair.Flags().StringVar(&keytype, "keytype", "", "Key type for generated key (RSA or AES) [required]")
	generateKeyPair.Flags().IntVar(&keysize, "keysize", 0, "Size of generated key (AES 128,192,256 - RSA 1024,2048,3072,4096) [required]")
	generateKeyPair.Flags().StringVar(&algorithm, "algorithm", "", "Algorithm to use, such as S256 [required]")
	generateKeyPair.MarkFlagRequired("label")
	generateKeyPair.MarkFlagRequired("keytype")
	generateKeyPair.MarkFlagRequired("keysize")
	generateKeyPair.MarkFlagRequired("algorithm")
}

func doGenerateKeyPair(cmd *cobra.Command) {

	var labelToUse string
	if cmd.Flags().Changed("label") {
		labelToUse = label
	}

	var keyIdToUse string
	if cmd.Flags().Changed("keyid") {
		keyIdToUse = keyid
	}

	var algorithmToUse string
	if cmd.Flags().Changed("algorithm") {
		algorithmToUse = algorithm
	}

	p11Token, err := p11.NewToken(p11Lib, p11TokenLabel, getPIN(cmd))
	handleError(err)

	defer p11Token.Finalise()
	handleError(p11Token.GenerateKeyPair(labelToUse, keyIdToUse, algorithmToUse, keytype, keysize))
}
