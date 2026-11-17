//go:build pkcs11

package pkcs11

/*
#cgo LDFLAGS: -ldl
#include <dlfcn.h>
#include <stdlib.h>
#include <string.h>
typedef unsigned long UL;
typedef struct { UL type; void* p; UL len; } ATTR;
typedef struct { UL mech; void* p; UL len; } MECH;
typedef struct { unsigned char* p; UL len; } SCALARP;
typedef UL (*dkp_t)(UL, MECH*, UL, ATTR*, UL, ATTR*, UL, UL*, UL*);
static void* sym(const char* lib, const char* name, void** handle) {
	void* h = dlopen(lib, RTLD_NOW | RTLD_NOLOAD);
	if (!h) h = dlopen(lib, RTLD_NOW);
	if (!h) return NULL;
	*handle = h;
	return dlsym(h, name);
}
static UL call_dkp(void* f, UL sess, UL mech, unsigned char* sc, UL sl, UL base,
	ATTR* pub, UL npub, ATTR* priv, UL npriv, UL* hpub, UL* hpriv) {
	SCALARP prm = { sc, sl };
	MECH m = { mech, &prm, sizeof(prm) };
	return ((dkp_t)f)(sess, &m, base, pub, npub, priv, npriv, hpub, hpriv);
}
*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/miekg/pkcs11"
)

// deriveKeyPair calls the Securosys vendor extension C_DeriveKeyPair.
// CKM_EC_SCALAR_CHILD_DERIVE is rejected by C_DeriveKey (CKR_MECHANISM_INVALID
// on firmware RX-3.4.0-T); the vendor function is the one that accepts
// CK_EC_SCALAR_CHILD_DERIVE_PARAMS {pScalar, ulScalarLen}.
// The C copy of scalar is wiped before this returns. The Go buffer is the caller's.
func deriveKeyPair(lib string, sess pkcs11.SessionHandle, mech uint, scalar []byte, base pkcs11.ObjectHandle, pubT, privT []*pkcs11.Attribute) (pkcs11.ObjectHandle, pkcs11.ObjectHandle, error) {
	if unsafe.Sizeof(C.UL(0)) != 8 {
		return 0, 0, fmt.Errorf("C_DeriveKeyPair shim requires a 64-bit CK_ULONG")
	}
	if len(scalar) == 0 {
		return 0, 0, fmt.Errorf("scalar is empty")
	}
	cl := C.CString(lib)
	defer C.free(unsafe.Pointer(cl))
	cn := C.CString("C_DeriveKeyPair")
	defer C.free(unsafe.Pointer(cn))
	var handle unsafe.Pointer
	f := C.sym(cl, cn, &handle)
	if handle != nil {
		defer C.dlclose(handle)
	}
	if f == nil {
		return 0, 0, fmt.Errorf("C_DeriveKeyPair symbol not found")
	}
	sc := C.CBytes(scalar)
	if sc == nil {
		return 0, 0, fmt.Errorf("scalar allocation failed")
	}
	defer func() {
		C.memset(sc, 0, C.size_t(len(scalar)))
		C.free(sc)
	}()
	pa, err := toCAttrs(pubT)
	if err != nil {
		return 0, 0, err
	}
	defer pa.free()
	pr, err := toCAttrs(privT)
	if err != nil {
		return 0, 0, err
	}
	defer pr.free()
	var hp, hv C.UL
	rv := C.call_dkp(f, C.UL(sess), C.UL(mech), (*C.uchar)(sc), C.UL(len(scalar)), C.UL(base),
		pa.arr, C.UL(pa.n), pr.arr, C.UL(pr.n), &hp, &hv)
	if rv != 0 {
		return 0, 0, pkcs11.Error(rv)
	}
	return pkcs11.ObjectHandle(hp), pkcs11.ObjectHandle(hv), nil
}

type cAttrs struct {
	arr   *C.ATTR
	n     int
	frees []unsafe.Pointer
}

func toCAttrs(as []*pkcs11.Attribute) (*cAttrs, error) {
	c := &cAttrs{n: len(as)}
	if len(as) == 0 {
		return c, nil
	}
	c.arr = (*C.ATTR)(C.calloc(C.size_t(len(as)), C.size_t(unsafe.Sizeof(C.ATTR{}))))
	if c.arr == nil {
		return nil, fmt.Errorf("attribute allocation failed")
	}
	sl := unsafe.Slice(c.arr, len(as))
	for i, a := range as {
		sl[i]._type = C.UL(a.Type)
		if len(a.Value) == 0 {
			continue
		}
		p := C.CBytes(a.Value)
		if p == nil {
			c.free()
			return nil, fmt.Errorf("attribute value allocation failed")
		}
		c.frees = append(c.frees, p)
		sl[i].p = p
		sl[i].len = C.UL(len(a.Value))
	}
	return c, nil
}

func (c *cAttrs) free() {
	if c == nil {
		return
	}
	for _, p := range c.frees {
		C.free(p)
	}
	if c.arr != nil {
		C.free(unsafe.Pointer(c.arr))
	}
}
