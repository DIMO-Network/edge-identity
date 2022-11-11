<!--
Copyright 2018 Thales UK Limited

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated
documentation files (the "Software"), to deal in the Software without restriction, including without limitation the
rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to
permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the
Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE
WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR
OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
-->

# edge-identity [![Go Report Card](https://goreportcard.com/badge/github.com/DIMO-Network/edge-identity)](https://goreportcard.com/report/github.com/DIMO-Network/edge-identity)

A command line tool for interacting with [PKCS&nbsp;#11 tokens](https://en.wikipedia.org/wiki/PKCS_11). The intended 
audience is developers writing PKCS&nbsp;#11 applications who need to inspect objects, import test keys, delete 
generated keys, etc. (We wrote this tool to help with our own development projects).

## Installation

```
go get -u github.com/DIMO-Network/edge-identity
```

## Usage With SoftHSM on Dawrin/OSX
Run `edge-identity --help` to see available commands. Run `edge-identity <command> --help` for help on individual commands.

## Usage With SoftHSM
```
brew install softhsm

or

sudo apt-get install softhsm 

and

sudo softhsm2-util  --init-token --slot 0 --label "dimo" --pin 1234
```

Run `edge-identity --help` to see available commands. Run `edge-identity <command> --help` for help on individual commands.

```
./edge-identity --lib /usr/lib/softhsm/libsofthsm2.so list  --token dimo --pin 1234

./edge-identity --lib /usr/lib/softhsm/libsofthsm2.so generateKeyPair --algorithm S256 --keytype EC --keysize 256 --label  clitest --token dimo  --pin 1234

./edge-identity --lib /usr/lib/softhsm/libsofthsm2.so getEthereumAddress --token dimo --label clitest --pin 1234

//For raw messages
./edge-identity --lib /usr/lib/softhsm/libsofthsm2.so sign --token dimo --label clitest --message "testmessage" --pin 1234

./edge-identity --lib /usr/lib/softhsm/libsofthsm2.so verify --token dimo --label clitest  --signature 0xeb3e92bc01b32e4f7cce5729fe6e7b91281f47bf1e78fcacf86a64a59c2ad4ce4c458f761b18a7f8d7bd44b9394a916e977c9e0b637537f0c69e15f5348152f901 --message "testmessage" --pin 1234

//For hashes computed outside of library
./edge-identity --lib /usr/lib/softhsm/libsofthsm2.so sign --token dimo --label clitest --hash "0x83206f8dc9e7119d9e063237f9a4f35a7b97af2192e219aeb761edd326391ced" --pin 1234

./edge-identity --lib /usr/lib/softhsm/libsofthsm2.so verify --token dimo --label clitest  --signature 0xd089c437525f44cbe9cdb9fed96b8d3a7e2856185621566a5118be1632adb55f7e47dc0d909f61f977f9c90fae792220446cef148a5d52e7cf09f789d226130a00 --hash "0x83206f8dc9e7119d9e063237f9a4f35a7b97af2192e219aeb761edd326391ced" --pin 1234
```

### Development
```
sudo softhsm2-util --delete-token --token dimo; sudo softhsm2-util  --init-token --slot 0 --label "dimo" --pin 1234 --so-pin 1234
```

### Release
```
GOARCH=arm GOOS=linux go build -ldflags="-s -w"; upx edge-identity
tar cvfz edge-identity-vX.X.X-linux-arm.tar.gz edge-identity
md5sum edge-identity-vX.X.X-linux-arm.tar.gz | cut -d ' ' -f 1 > edge-identity-vX.X.X-linux-arm.tar.gz.md5
```