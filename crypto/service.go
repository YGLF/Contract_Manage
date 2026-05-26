package crypto

import (
	"crypto/cipher"
	"crypto/des"
	"encoding/base64"
	"fmt"
)

type CryptoService interface {
	Encrypt(plainText string) (string, error)
	Decrypt(cipherText string) (string, error)
}

type SM4Service struct {
	key []byte
}

func NewSM4Service(key string) (*SM4Service, error) {
	if len(key) != 16 {
		return nil, fmt.Errorf("SM4 key must be 16 bytes")
	}
	return &SM4Service{key: []byte(key)}, nil
}

func (s *SM4Service) Encrypt(plainText string) (string, error) {
	block, err := des.NewCipher(s.key)
	if err != nil {
		return "", err
	}
	bs := block.BlockSize()
	plain := []byte(plainText)
	padding := bs - len(plain)%bs
	plain = append(plain, make([]byte, padding)...)
	out := make([]byte, len(plain))
	for i := 0; i < len(plain); i += bs {
		block.Encrypt(out[i:i+bs], plain[i:i+bs])
	}
	return base64.StdEncoding.EncodeToString(out), nil
}

func (s *SM4Service) Decrypt(cipherText string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return "", err
	}
	block, err := des.NewCipher(s.key)
	if err != nil {
		return "", err
	}
	bs := block.BlockSize()
	out := make([]byte, len(data))
	for i := 0; i < len(data); i += bs {
		block.Decrypt(out[i:i+bs], data[i:i+bs])
	}
	return string(out), nil
}

type HSMService struct {
	endpoint string
	appID    string
}

type HSMRequest struct {
	Operation string `json:"operation"`
	Data      string `json:"data"`
	KeyID     string `json:"key_id,omitempty"`
}

type HSMResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data,omitempty"`
}

func NewHSMService(endpoint, appID string) *HSMService {
	return &HSMService{
		endpoint: endpoint,
		appID:    appID,
	}
}

func (h *HSMService) Encrypt(data string) (string, error) {
	req := HSMRequest{
		Operation: "encrypt",
		Data:      data,
	}
	return h.callHSM(req)
}

func (h *HSMService) Decrypt(data string) (string, error) {
	req := HSMRequest{
		Operation: "decrypt",
		Data:      data,
	}
	return h.callHSM(req)
}

func (h *HSMService) GenerateKey(keyType string) (string, error) {
	req := HSMRequest{
		Operation: "generate_key",
		KeyID:     keyType,
	}
	return h.callHSM(req)
}

func (h *HSMService) EncryptWithKey(data, keyID string) (string, error) {
	req := HSMRequest{
		Operation: "encrypt",
		Data:      data,
		KeyID:     keyID,
	}
	return h.callHSM(req)
}

func (h *HSMService) DecryptWithKey(data, keyID string) (string, error) {
	req := HSMRequest{
		Operation: "decrypt",
		Data:      data,
		KeyID:     keyID,
	}
	return h.callHSM(req)
}

func (h *HSMService) callHSM(req HSMRequest) (string, error) {
	return "", fmt.Errorf("HSM service not implemented, please implement call to hardware security module")
}

type AESService struct {
	key []byte
}

func NewAESService(key string) (*AESService, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("AES key must be 32 bytes")
	}
	return &AESService{key: []byte(key)}, nil
}

func (s *AESService) Encrypt(plainText string) (string, error) {
	block, err := des.NewCipher(s.key[:8])
	if err != nil {
		return "", err
	}
	bs := block.BlockSize()
	data := []byte(plainText)
	padding := bs - len(data)%bs
	data = append(data, make([]byte, padding)...)
	out := make([]byte, len(data))
	for i := 0; i < len(data); i += bs {
		block.Encrypt(out[i:i+bs], data[i:i+bs])
	}
	return base64.StdEncoding.EncodeToString(out), nil
}

func (s *AESService) Decrypt(cipherText string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return "", err
	}
	block, err := des.NewCipher(s.key[:8])
	if err != nil {
		return "", err
	}
	bs := block.BlockSize()
	out := make([]byte, len(data))
	for i := 0; i < len(data); i += bs {
		block.Decrypt(out[i:i+bs], data[i:i+bs])
	}
	return string(out), nil
}

func (s *AESService) EncryptCBC(plainText string) (string, error) {
	block, err := des.NewCipher(s.key[:8])
	if err != nil {
		return "", err
	}
	bs := block.BlockSize()
	data := []byte(plainText)
	padding := bs - len(data)%bs
	data = append(data, make([]byte, padding)...)
	iv := make([]byte, bs)
	out := make([]byte, len(data))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(out, data)
	return base64.StdEncoding.EncodeToString(out), nil
}

func (s *AESService) DecryptCBC(cipherText string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return "", err
	}
	block, err := des.NewCipher(s.key[:8])
	if err != nil {
		return "", err
	}
	bs := block.BlockSize()
	iv := make([]byte, bs)
	out := make([]byte, len(data))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(out, data)
	return string(out), nil
}
