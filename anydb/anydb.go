// Copyright © 2023 aerth
// Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the “Software”), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:
// The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.
// THE SOFTWARE IS PROVIDED “AS IS”, WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package anydb

import (
	"fmt"
	"log"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/aerth/mostly/ncode"
	"go.etcd.io/bbolt"
)

type byteslike interface {
	~string | ~[]byte
}

// FetchDB anything magic
func FetchDB[T any, K byteslike](db *bbolt.DB, bucket string, key ...K) (T, error) {
	var v T
	err := db.View(func(tx *bbolt.Tx) error {
		var err error
		v, err = FetchDB_Tx[T](tx, bucket, key...)
		return err
	})
	return v, err
}

// FetchDB_Tx anything (but in a Tx)
func FetchDB_Tx[T any, K byteslike](tx *bbolt.Tx, bucket string, key ...K) (T, error) {
	bu := tx.Bucket([]byte(bucket))
	if bu == nil {
		var v T
		return v, bbolt.ErrBucketNotFound
	}
	if ncode.DebugJsonRequests {
		var caller string
		for i := 1; i <= 6; i++ {
			_, file, num, ok := runtime.Caller(i)
			if !ok {
				break
			}
			fname := filepath.Base(file)
			if strings.HasPrefix(fname, "asm_") || strings.HasPrefix(fname, "xxxx_some_other_name_prefix") {
				break
			}
			caller += fmt.Sprintf("%s:%d ", fname, num)
		}
		// log.Println(caller, "serving error:", err)
		log.Println(caller, "fetchdb: read", bucket, string(key[0]))
	}
	l := len(key)
	if l == 0 {
		var v T
		return v, fmt.Errorf("no key?")
	}
	if l == 1 {
		if len(key[0]) == 0 {
			var v T
			return v, fmt.Errorf("empty key?")
		}
		return ncode.DecodeJson[T](bu.Get([]byte(key[0])))
	}
	log.Println("checking", l, "nested", string(key[0]), string(key[1]))
	for i := 0; i < l-1; i++ {
		log.Printf("checking: %q (hex: %02x)", string(key[i]), key[i])
		bu = bu.Bucket([]byte(key[i]))
		if bu == nil {
			var v T
			log.Printf("fail %d: bucket %s is nil", i, string(key[i]))
			return v, fmt.Errorf("bad nested lookup")
		}
	}
	return ncode.DecodeJson[T](bu.Get([]byte(key[l-1])))

}

func Update[T any, K byteslike](db *bbolt.DB, bucket string, key K, modifier func(v T) (T, error)) error {
	return db.Update(func(tx *bbolt.Tx) error {
		return UpdateTx(tx, bucket, key, modifier)
	})
}
func UpdateNested[T any, K byteslike](db *bbolt.DB, bucket string, key []K, modifier func(v T) (T, error)) error {
	return db.Update(func(tx *bbolt.Tx) error {
		return UpdateTxNested(tx, bucket, key, modifier)
	})
}
func UpdateTx[T any, K byteslike](tx *bbolt.Tx, bucket string, key K, modifier func(v T) (T, error)) error {

	got, err := FetchDB_Tx[T](tx, bucket, key)
	if err != nil {
		return err
	}
	got, err = modifier(got)
	if err != nil {
		return err
	}
	return StoreDB_Tx(tx, bucket, key, got)
}
func UpdateTxNested[T any, K byteslike](tx *bbolt.Tx, bucket string, key []K, modifier func(v T) (T, error)) error {

	got, err := FetchDB_Tx[T](tx, bucket, key...)
	if err != nil {
		return err
	}
	got, err = modifier(got)
	if err != nil {
		return err
	}
	return StoreDBNested_Tx(tx, bucket, key, got)
}
func StoreDB[K byteslike](db *bbolt.DB, bucket string, key K, val any) error {
	return db.Update(func(tx *bbolt.Tx) error {
		return StoreDB_Tx(tx, bucket, key, val)
	})
}
func StoreDB_Tx[K byteslike](tx *bbolt.Tx, bucket string, key K, val any) error {
	bu := tx.Bucket([]byte(bucket))
	if bu == nil {
		return bbolt.ErrBucketNotFound
	}
	return bu.Put([]byte(key), ncode.Json(val))
}
func StoreDBNested[K byteslike](db *bbolt.DB, bucket string, key []K, val any) error {
	return db.Update(func(tx *bbolt.Tx) error {
		return StoreDBNested_Tx(tx, bucket, key, val)
	})
}
func StoreDBNested_Tx[K byteslike](tx *bbolt.Tx, bucket string, key []K, val any) error {
	bu := tx.Bucket([]byte(bucket))
	if bu == nil {
		return bbolt.ErrBucketNotFound
	}
	l := len(key)
	if l == 0 {
		// should panic really
		return ncode.ErrZeroLength
	}
	for i := 0; i < l-1; i++ {
		bu = bu.Bucket([]byte(key[i]))
		if bu == nil {
			log.Printf("fail %d: bucket %s is nil", i, string(key[i]))
			return fmt.Errorf("bad nested lookup")
		}
	}
	return bu.Put([]byte(key[l-1]), ncode.Json(val))
}
