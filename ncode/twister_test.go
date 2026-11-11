package ncode

import (
	"net/url"
	"testing"
)

func TestTwister1(t *testing.T) {
	t.Run("twister urls", func(t *testing.T) {
		tw := Twister(url.Parse)
		urls, err := tw.Twist([]string{"https://github.com", "https://google.com"})
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range urls {
			v.Path = "/search"
			v.RawQuery = "q=foo"
			println(v.String())
		}
	})
}
