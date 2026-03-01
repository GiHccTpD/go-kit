package crypto

import (
	"bytes"
	"crypto"
	"crypto/des"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/marspere/goencrypt"
	"log"
	"net/url"
	"strings"
)

func MD5(src string) string {
	return MD5Byte([]byte(src))
}

func MD5Byte(src []byte) string {
	md5Ctx := md5.New()
	_, err := md5Ctx.Write(src)
	if err != nil {
		return ""
	}

	return strings.ToUpper(hex.EncodeToString(md5Ctx.Sum(nil)))
}

func SHA256(str string) string {
	res, err := sha256Func([]byte(str))
	if err != nil {
		return ""
	}
	return hex.EncodeToString(res)
}

func SHA256WithSecret(secret []byte, data []byte) (string, error) {
	h := hmac.New(sha256.New, secret)
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil)), nil
}

func sha256Func(data []byte) ([]byte, error) {
	sha256Ctx := sha256.New()
	_, err := sha256Ctx.Write(data)
	if err != nil {
		return nil, err
	}
	return sha256Ctx.Sum(nil), nil
}

func sha1Func(data []byte) ([]byte, error) {
	sha1Ctx := sha1.New()
	_, err := sha1Ctx.Write(data)
	if err != nil {
		return nil, err
	}
	return sha1Ctx.Sum(nil), nil
}

func SHA1withRSASign(privateKey []byte, data []byte) ([]byte, error) {
	block, _ := pem.Decode(privateKey)
	if block == nil {
		return nil, errors.New("private key error!")
	}
	priv, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	digist, err := sha1Func(data)
	if err != nil {
		return nil, err
	}
	return rsa.SignPKCS1v15(rand.Reader, priv.(*rsa.PrivateKey), crypto.SHA1, digist)
}
func MD5withRSASign(privateKey []byte, data []byte) ([]byte, error) {
	block, _ := pem.Decode(privateKey)
	if block == nil {
		return nil, errors.New("private key error!")
	}
	priv, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	md5Ctx := md5.New()
	_, err = md5Ctx.Write(data)
	if err != nil {
		return nil, err
	}

	digist := md5Ctx.Sum(nil)

	fmt.Println(hex.EncodeToString(digist))

	return rsa.SignPKCS1v15(rand.Reader, priv.(*rsa.PrivateKey), crypto.MD5, digist)
}

func SHA256withRSASign(privateKey []byte, data []byte) ([]byte, error) {
	block, _ := pem.Decode(privateKey)
	if block == nil {
		return nil, errors.New("private key error!")
	}
	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	digist, err := sha256Func(data)
	if err != nil {
		return nil, err
	}

	log.Println(Base64Encode(digist))

	return rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, digist)
}

func SHA256withRSAVerify(publicKey []byte, sig []byte, data []byte) (bool, error) {
	block, _ := pem.Decode(publicKey)

	if block == nil {
		return false, errors.New("public key error")
	}

	pubInterface, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		return false, err
	}

	hashed, err := sha256Func(data)
	if err != nil {
		return false, err
	}

	err = rsa.VerifyPKCS1v15(pubInterface, crypto.SHA256, hashed[:], sig)

	return err == nil, err
}

func Base64Decode(src string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(src)
}

func Base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func GenerateRsaKeyPairWithSize(keySize int) (string, string, error) {
	reader := rand.Reader
	bitSize := keySize

	key, err := rsa.GenerateKey(reader, bitSize)
	if err != nil {
		return "", "", err
	}

	publicKey := key.PublicKey

	privateRes, err := savePEMKey(key)
	if err != nil {
		return "", "", err
	}
	publicRes, err := savePublicPEMKey(publicKey)
	if err != nil {
		return "", "", err
	}
	return publicRes, privateRes, nil
}

func GenerateRsaKeyPairWithPKCS8(bits int) ([]byte, []byte, error) {

	// 生成私钥文件
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)

	if err != nil {
		return nil, nil, err
	}
	derStream, _ := x509.MarshalPKCS8PrivateKey(privateKey)
	priBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: derStream,
	}

	priKey := pem.EncodeToMemory(priBlock)
	// 生成公钥文件
	publicKey := &privateKey.PublicKey
	derPkix, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, nil, err
	}
	publicBlock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: derPkix,
	}

	pubKey := pem.EncodeToMemory(publicBlock)

	if err != nil {
		return nil, nil, err
	}
	return pubKey, priKey, nil
}

func GenerateRsaKeyPair() (string, string, error) {
	return GenerateRsaKeyPairWithSize(1024)
}

func savePEMKey(key *rsa.PrivateKey) (string, error) {

	var privateKey = &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}

	var buffer bytes.Buffer
	err := pem.Encode(&buffer, privateKey)
	if err != nil {
		return "", err
	}
	return buffer.String(), nil

}

func savePublicPEMKey(pubkey rsa.PublicKey) (string, error) {
	asn1Bytes, err := asn1.Marshal(pubkey)

	var pemkey = &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: asn1Bytes,
	}
	var buffer bytes.Buffer
	err = pem.Encode(&buffer, pemkey)
	if err != nil {
		return "", err
	}
	return buffer.String(), nil
}
func RSADecryptWithPKCS8(data []byte, privateKey []byte) ([]byte, error) {
	block, _ := pem.Decode(privateKey)
	if block == nil {
		return nil, errors.New("private key error!")
	}
	priv, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return rsa.DecryptPKCS1v15(rand.Reader, priv.(*rsa.PrivateKey), data)
}
func RSADecryptWithPKCS1(data []byte, privateKey []byte) ([]byte, error) {
	block, _ := pem.Decode(privateKey)
	if block == nil {
		return nil, errors.New("private key error!")
	}
	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return rsa.DecryptPKCS1v15(rand.Reader, priv, data)
}

func RSAEncryptWithPKCS1(data []byte, publicKey []byte) ([]byte, error) {
	block, _ := pem.Decode(publicKey)
	if block == nil {
		return nil, errors.New("private key error!")
	}
	priv, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return rsa.EncryptPKCS1v15(rand.Reader, priv, data)
}

func RSAEncrypt(data []byte, publicKey []byte) ([]byte, error) {
	block, _ := pem.Decode(publicKey)
	if block == nil {
		return nil, errors.New("public key error")

	}
	pubInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	pub := pubInterface.(*rsa.PublicKey)
	return rsa.EncryptPKCS1v15(rand.Reader, pub, data)
}

func DESEncrypt(data string, key string) (string, error) {

	urlData := url.QueryEscape(data)
	baseData := base64.StdEncoding.EncodeToString([]byte(urlData))
	desCiper := goencrypt.NewDESCipher([]byte(key), []byte(key), goencrypt.CBCMode, goencrypt.Pkcs7, goencrypt.PrintBase64)
	encData, err := desCiper.DESEncrypt([]byte(baseData))
	if err != nil {
		return data, err
	}
	realData, _ := base64.StdEncoding.DecodeString(encData)
	finalData := strings.ToUpper(hex.EncodeToString(realData))
	return finalData, nil

}

func DESECBEncryptHex(data string, key string) (string, error) {

	desCiper := goencrypt.NewDESCipher([]byte(key), []byte(key), goencrypt.ECBMode, goencrypt.Pkcs7, goencrypt.PrintHex)
	data, err := desCiper.DESEncrypt([]byte(data))
	if err != nil {
		fmt.Println(err)
	}
	return data, err
}

func DESECBDecryptHex(data string, key string) (string, error) {

	desCiper := goencrypt.NewDESCipher([]byte(key), []byte(key), goencrypt.ECBMode, goencrypt.Pkcs7, goencrypt.PrintHex)
	data, err := desCiper.DESDecrypt(data)
	if err != nil {
		fmt.Println(err)
	}
	return data, err

}

func DESECBDecryptBase64(data string, key string) (string, error) {

	desCiper := goencrypt.NewDESCipher([]byte(key), []byte(key), goencrypt.ECBMode, 1, goencrypt.PrintBase64)
	data, err := desCiper.DESEncrypt([]byte(data))
	if err != nil {
		fmt.Println(err)
	}
	return data, err
}

func DESECBEncryptBase64(data string, key string) (string, error) {

	desCiper := goencrypt.NewDESCipher([]byte(key), []byte(key), goencrypt.ECBMode, 1, goencrypt.PrintBase64)
	data, err := desCiper.DESEncrypt([]byte(data))
	if err != nil {
		fmt.Println(err)
	}
	return data, err
}

func MyEncrypt(data, key []byte) ([]byte, error) {
	block, err := des.NewCipher(key)
	if err != nil {
		return nil, err
	}
	bs := block.BlockSize()
	data = PKCS5Padding(data, bs)
	if len(data)%bs != 0 {
		return nil, errors.New("Need a multiple of the blocksize")
	}
	out := make([]byte, len(data))
	dst := out
	for len(data) > 0 {
		block.Encrypt(dst, data[:bs])
		data = data[bs:]
		dst = dst[bs:]
	}
	return out, nil
}

func PKCS5Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

func HmacSha1(keyStr, value string) string {
	key := []byte(keyStr)
	mac := hmac.New(sha1.New, key)
	mac.Write([]byte(value))
	res := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return res
}

func Test() {
	res, err := hex.DecodeString("4F6B495E98DA9749")
	if err != nil {
		fmt.Println(err)
	}
	eRes, err := MyEncrypt([]byte("123123123"), res)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(string(eRes))
}
