package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const Prefix = "enc:v1:"

type Vault struct{ cipher cipher.AEAD }

// Reuse the existing project-test key so upgrading preserves saved tasks.
func Open(directory string) (*Vault, error) {
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	path := filepath.Join(directory, "project-test.key")
	key, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		key = make([]byte, 32)
		if _, err = rand.Read(key); err != nil {
			return nil, err
		}
		file, createErr := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if os.IsExist(createErr) {
			key, err = os.ReadFile(path)
		} else if createErr != nil {
			return nil, createErr
		} else {
			_, err = file.Write(key)
			closeErr := file.Close()
			if err == nil {
				err = closeErr
			}
		}
	}
	if err != nil {
		return nil, fmt.Errorf("平台加密密钥读取失败")
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("平台加密密钥必须为32字节，请恢复原始密钥")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Vault{cipher: aead}, nil
}

func (v *Vault) Seal(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	nonce := make([]byte, v.cipher.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := v.cipher.Seal(nonce, nonce, []byte(value), []byte(Prefix))
	return Prefix + base64.StdEncoding.EncodeToString(sealed), nil
}

func (v *Vault) Unseal(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if !strings.HasPrefix(value, Prefix) {
		return "", fmt.Errorf("模型密钥尚未完成加密迁移")
	}
	bytes, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, Prefix))
	if err != nil || len(bytes) < v.cipher.NonceSize() {
		return "", fmt.Errorf("模型密钥密文无效")
	}
	n := v.cipher.NonceSize()
	plain, err := v.cipher.Open(nil, bytes[:n], bytes[n:], []byte(Prefix))
	if err != nil {
		return "", fmt.Errorf("模型密钥解密失败，请恢复与数据库匹配的原始密钥")
	}
	return string(plain), nil
}

// Authenticate old task/private-context capsules without changing their
// format, returning their contents, or logging credentials.
func (v *Vault) CheckLegacy(value string) error {
	if value == "" {
		return nil
	}
	data, err := base64.StdEncoding.DecodeString(value)
	n := v.cipher.NonceSize()
	if err != nil || len(data) < n {
		return fmt.Errorf("旧项目任务密文无效")
	}
	if _, err = v.cipher.Open(nil, data[:n], data[n:], nil); err != nil {
		return fmt.Errorf("旧项目任务密钥不匹配，请恢复原始 project-test.key")
	}
	return nil
}
