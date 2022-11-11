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
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/cobra"
)

// generateCmd represents the generate command
var getEthereumAddress = &cobra.Command{
	Use:   "getEthereumAddress",
	Short: "Get Ethereum Address of an EC Key",
	Run: func(cmd *cobra.Command, args []string) {
		doGetEthereumAddress(cmd)
	},
}

var keyid string

func init() {
	rootCmd.AddCommand(getEthereumAddress)

	getEthereumAddress.Flags().StringVar(&label, "label", "", "Label for generated key [required]")
	getEthereumAddress.Flags().StringVar(&keyid, "keyid", "", "KeyId for generated key [required]")
	getEthereumAddress.MarkFlagRequired("label")
}

func doGetEthereumAddress(cmd *cobra.Command) {

	var labelToUse string
	if cmd.Flags().Changed("label") {
		labelToUse = label
	}

	var keyIdToUse string
	if cmd.Flags().Changed("keyid") {
		keyIdToUse = keyid
	}

	p11Token, err := p11.NewToken(p11Lib, p11TokenLabel, getPIN(cmd))
	handleError(err)
	defer p11Token.Finalise()
	pubKey, _, err := p11Token.GetPublicKey(labelToUse, keyIdToUse)
	handleError(err)
	addr := crypto.PubkeyToAddress(*pubKey)
	log.Println("Address:", addr)

}
