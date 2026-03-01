package smx

//#cgo CFLAGS:-I/home/fan/workspace/C++/TASM2-COPY/include -I/home/fan/Tools/TASSL-1.1.1b/normal-build/include
//#cgo LDFLAGS:-L/home/fan/workspace/C++/TASM2-COPY/dist -lICitySMX
//#include "sm2.h"
//#include "sm3.h"
//#include <stdlib.h>
import "C"

import (
	"fmt"
	"unsafe"
)

func ICitySM3(source []byte) string {
	cSource := (*C.char)(unsafe.Pointer(&source[0]))
	var cRes *C.char
	defer func() {
		if cRes != nil {
			C.free(unsafe.Pointer(cRes))
		}
	}()

	ret := C.icity_sm3_digest(cSource, C.int(len(source)), &cRes)

	digest := ""
	if ret == 0 {
		digest = C.GoString(cRes)
	}
	return digest
}

func CreateKeyPair() (string, string, error) {
	var cPubKey *C.char
	var cPrivateKey *C.char
	defer func() {
		if cPubKey != nil {
			C.free(unsafe.Pointer(cPubKey))
		}
		if cPrivateKey != nil {
			C.free(unsafe.Pointer(cPrivateKey))
		}
	}()

	ret := C.create_key_pair(&cPubKey, &cPrivateKey)
	pubKey := ""
	privateKey := ""
	if ret == 0 {
		pubKey = C.GoString(cPubKey)
		privateKey = C.GoString(cPrivateKey)
		return pubKey, privateKey, nil
	}
	return pubKey, privateKey, fmt.Errorf("create key pair error ret = %d", int(ret))
}

func ICitySM2Encrypt(source []byte, pubKey string) (string, error) {

	cPubKey := C.CString(pubKey)
	defer C.free(unsafe.Pointer(cPubKey))
	cSource := (*C.char)(unsafe.Pointer(&source[0]))

	var cEncryptStr *C.char
	defer func() {
		if cEncryptStr != nil {
			C.free(unsafe.Pointer(cEncryptStr))
		}
	}()

	ret := C.icity_sm2_encrypt(cPubKey, cSource, C.int(len(source)), &cEncryptStr)

	encryptStr := ""
	if ret == 0 {
		encryptStr = C.GoString(cEncryptStr)
		return encryptStr, nil
	}
	return encryptStr, fmt.Errorf("encrypt error ret : %d", int(ret))
}

func ICitySM2Decrypt(encryptData []byte, privateKey string) ([]byte, error) {
	cPrivateKey := C.CString(privateKey)
	defer C.free(unsafe.Pointer(cPrivateKey))
	cEncryptData := (*C.char)(unsafe.Pointer(&encryptData[0]))

	var cClearStr *C.char
	defer func() {
		if cClearStr != nil {
			C.free(unsafe.Pointer(cClearStr))
		}
	}()
	clearDataLength := C.int(0)
	ret := C.icity_sm2_decrypt(cPrivateKey, cEncryptData, &cClearStr, &clearDataLength)

	if ret == 0 {
		goMsg := C.GoBytes(unsafe.Pointer(cClearStr), C.int(clearDataLength))
		return goMsg, nil
	}
	return nil, fmt.Errorf("decrypt error ret:%d\n", int(ret))
}

func ICitySM2Sign(signData []byte, privateKey string) (string, error) {
	cPrivateKey := C.CString(privateKey)
	defer C.free(unsafe.Pointer(cPrivateKey))

	cSignData := (*C.char)(unsafe.Pointer(&signData[0]))

	var cSignStr *C.char
	defer func() {
		if cSignStr != nil {
			C.free(unsafe.Pointer(cSignStr))
		}
	}()
	ret := C.icity_sm2_sign(cPrivateKey, cSignData, C.int(len(signData)), &cSignStr)
	signStr := ""
	if ret == 0 {
		signStr = C.GoString(cSignStr)
		return signStr, nil
	}
	return signStr, fmt.Errorf("sign error ret:%d", int(ret))
}

func ICitySM2Verify(signData []byte, pubKey string, signature string) (bool, error) {
	cPubKey := C.CString(pubKey)
	cSignature := C.CString(signature)
	defer C.free(unsafe.Pointer(cPubKey))
	cSignData := (*C.char)(unsafe.Pointer(&signData[0]))

	ret := C.icity_sm2_verify(cPubKey, cSignData, C.int(len(signData)), cSignature)
	return ret == 0, nil
}
