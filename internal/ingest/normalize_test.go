// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package ingest_test

import (
	"testing"

	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/ingest"
	"golang.org/x/text/unicode/norm"
)

func TestNormalization_UnicodeAndArabic(t *testing.T) {
	t.Run("TC-UNICODE-01: Arabic title with diacritics stripped", func(t *testing.T) {
		// "حَاسُوبٌ مَحْمُولٌ" with fatha, damma, sukun, tanwin damma
		input := "حَاسُوبٌ مَحْمُولٌ"
		expected := "حاسوب محمول"
		got := ingest.NormalizeArabicForSearch(input)
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("TC-UNICODE-01b: Tatweel stripped", func(t *testing.T) {
		input := "حــــاســـوب"
		expected := "حاسوب"
		got := ingest.NormalizeArabicForSearch(input)
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("TC-UNICODE-02: Arabic title with alef variants unified", func(t *testing.T) {
		// أ إ آ ٱ -> ا
		input := "أفضل إمكانات آبل ٱستخدام"
		expected := "افضل امكانات ابل استخدام"
		got := ingest.NormalizeArabicForSearch(input)
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("TC-UNICODE-02b: Ta Marbuta and Alef Maksura unified", func(t *testing.T) {
		// ة -> ه, ى -> ي
		input := "شاشة ذكية فائقة على"
		expected := "شاشه ذكيه فائقه علي"
		got := ingest.NormalizeArabicForSearch(input)
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("TC-UNICODE-03: Title in NFD form converted to NFC", func(t *testing.T) {
		// é in NFD is e + combining acute (U+0301)
		nfd := norm.NFD.String("café")
		if norm.NFC.IsNormalString(nfd) {
			t.Fatalf("expected test input to not be NFC")
		}
		nfc := ingest.NormalizeNFC(nfd)
		if !norm.NFC.IsNormalString(nfc) {
			t.Errorf("expected result to be NFC normalized")
		}
		if nfc != "café" {
			t.Errorf("expected %q, got %q", "café", nfc)
		}
	})

	t.Run("TC-UNICODE-04: Emoji and bidirectional markers preserved", func(t *testing.T) {
		input := "💻 Laptop \u200F(عربي)\u200E ⚡"
		got := ingest.NormalizeNFC(input)
		if got != input {
			t.Errorf("expected emoji and RTL marks preserved, got %q", got)
		}
	})

	t.Run("TC-UNICODE-05: SQL metacharacters stored literally", func(t *testing.T) {
		input := "'; DROP TABLE variants; --"
		got := ingest.NormalizeNFC(input)
		if got != input {
			t.Errorf("expected SQL metacharacters preserved literally, got %q", got)
		}
	})
}
