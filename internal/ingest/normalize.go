// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package ingest

import (
	"strings"

	"golang.org/x/text/unicode/norm"
)

// NormalizeNFC converts an arbitrary string to Unicode NFC form.
func NormalizeNFC(s string) string {
	return norm.NFC.String(s)
}

// NormalizeArabicForSearch strips diacritics and tatweel, and normalizes alef, ta-marbuta, and ya forms.
func NormalizeArabicForSearch(s string) string {
	s = norm.NFC.String(s)
	var b strings.Builder
	b.Grow(len(s))

	for _, r := range s {
		switch r {
		// Tashkeel (diacritics) & Tatweel
		case '\u064B', '\u064C', '\u064D', '\u064E', '\u064F', '\u0650', '\u0651', '\u0652', '\u0670', '\u0640':
			continue

		// Alef normalization: آ, أ, إ, ٱ -> ا
		case '\u0622', '\u0623', '\u0625', '\u0671':
			b.WriteRune('\u0627')

		// Ta Marbuta: ة -> ه
		case '\u0629':
			b.WriteRune('\u0647')

		// Alef Maksura: ى -> ي
		case '\u0649':
			b.WriteRune('\u064A')

		default:
			b.WriteRune(r)
		}
	}

	return b.String()
}
