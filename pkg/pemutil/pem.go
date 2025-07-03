/*
Copyright 2025 The maco Authors

This program is offered under a commercial and under the AGPL license.
For AGPL licensing, see below.

AGPL licensing:
This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package pemutil

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
)

type RsaPair struct {
	Private []byte
	Public  []byte
}

// GenerateRSA generates rsa key pair
func GenerateRSA(bits int, logo string) (*RsaPair, error) {
	if bits%2048 != 0 {
		return nil, fmt.Errorf("bits must be a multiple of 2048")
	}

	// generate rsa key
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, fmt.Errorf("generate rsa key: %w", err)
	}

	// import private key
	publicKey := &privateKey.PublicKey

	if logo != "" {
		logo = "RSA"
	}
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privatePEM := pem.EncodeToMemory(&pem.Block{
		Type:  logo + " PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("marshal public key: %w", err)
	}
	publicPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	pair := &RsaPair{
		Private: privatePEM,
		Public:  publicPEM,
	}

	return pair, nil
}

// EncodeByRSA 使用RSA公钥加密数据，支持长文本分段加密
func EncodeByRSA(plaintext, publicKey []byte) ([]byte, error) {
	// 解析PEM格式公钥
	block, _ := pem.Decode(publicKey)
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, errors.New("invalid PEM format or key type")
	}

	// 兼容解析PKIX和PKCS1格式公钥
	pubInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		// 尝试PKCS1格式解析
		if pub, err2 := x509.ParsePKCS1PublicKey(block.Bytes); err2 == nil {
			return encryptChunks(pub, plaintext)
		}
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	pub, ok := pubInterface.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("not an RSA public key")
	}
	return encryptChunks(pub, plaintext)
}

// DecodeByRSA 使用RSA私钥解密数据
func DecodeByRSA(ciphertext, privateKey []byte) ([]byte, error) {
	block, _ := pem.Decode(privateKey)
	if block == nil {
		return nil, errors.New("invalid PEM data")
	}

	// 支持PKCS1和PKCS8格式私钥
	var priv *rsa.PrivateKey
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		priv = key
	} else if key2, err2 := x509.ParsePKCS8PrivateKey(block.Bytes); err2 == nil {
		rsaKey, ok := key2.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("not an RSA private key")
		}
		priv = rsaKey
	} else {
		return nil, fmt.Errorf("unsupported private key format: %w", err)
	}

	// 计算最大解密块大小
	chunkSize := priv.Size()
	var plaintext []byte

	for offset := 0; offset < len(ciphertext); offset += chunkSize {
		end := offset + chunkSize
		if end > len(ciphertext) {
			end = len(ciphertext)
		}

		chunk, err := rsa.DecryptPKCS1v15(rand.Reader, priv, ciphertext[offset:end])
		if err != nil {
			return nil, fmt.Errorf("decryption failed at offset %d: %w", offset, err)
		}
		plaintext = append(plaintext, chunk...)
	}
	return plaintext, nil
}

// encryptChunks 分段加密处理（解决RSA加密长度限制）
func encryptChunks(pub *rsa.PublicKey, data []byte) ([]byte, error) {
	// 计算单次加密最大长度（PKCS1v15填充占用11字节）
	maxChunkSize := pub.Size() - 11
	var ciphertext []byte

	for offset := 0; offset < len(data); offset += maxChunkSize {
		end := offset + maxChunkSize
		if end > len(data) {
			end = len(data)
		}

		chunk, err := rsa.EncryptPKCS1v15(rand.Reader, pub, data[offset:end])
		if err != nil {
			return nil, fmt.Errorf("encryption failed at offset %d: %w", offset, err)
		}
		ciphertext = append(ciphertext, chunk...)
	}
	return ciphertext, nil
}
