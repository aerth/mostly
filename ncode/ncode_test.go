package ncode_test

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"testing"

	"github.com/aerth/mostly/ncode"
)

func TestNcode1(t *testing.T) {
	t.Run("urls", func(t *testing.T) {
		log := log.New(os.Stderr, "", 0)
		urls, err := ncode.TwistParsePtr([]string{"https://google.com", "https://myspace.com"}, url.Parse)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range urls {
			log.Printf("scheme=%q host=%q", v.Scheme, v.Host)
		}
	})
	t.Run("atoi", func(t *testing.T) {
		num := []string{"1", "12", "123", "1234", "1234567812345678"}
		numbers, err := ncode.TwistParse(num, strconv.Atoi)
		if err != nil {
			t.Error(err)
		}
		for i, n := range num {
			if n != fmt.Sprint(numbers[i]) {
				t.Fatal(i, n, numbers[i])
			}
		}
	})

	t.Run("bools", func(t *testing.T) {
		bools := []string{"true", "false", "1", "0", "True", "False", "TRUE", "FALSE"}
		results, err := ncode.TwistParse(bools, strconv.ParseBool)
		if err != nil {
			t.Errorf("%+v", err)
		}
		for i, v := range bools {
			if v != fmt.Sprint(results[i]) {
				t.Fatal(i, v, results[i])
			}
		}
	})
}
