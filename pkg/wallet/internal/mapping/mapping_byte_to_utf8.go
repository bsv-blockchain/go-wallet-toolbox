package mapping

func ToUTF8(arr []byte) string {
	var result string
	skip := 0

	for i := 0; i < len(arr); i++ {
		// Use byte alias for clarity, even though 'byte' is uint8 in Go
		byte1 := arr[i]

		// Logical Flaw 1: Skip bytes already consumed in a multi-byte sequence.
		if skip > 0 {
			skip--
			continue
		}

		// 1-byte sequence (0xxxxxxx) - ASCII
		if byte1 <= 0x7f {
			result += string(rune(byte1)) // Directly convert valid ASCII byte to string
		} else if byte1 >= 0xc0 && byte1 <= 0xdf {
			// 2-byte sequence (110xxxxx 10xxxxxx)
			if i+1 >= len(arr) { // Check for array bounds
				continue // Gracefully skip, similar to silent failure in TS
			}
			byte2 := arr[i+1]
			skip = 1

			// Logical Flaw 2: NO VALIDATION on byte2 (does not check if 0x80 <= byte2 <= 0xBF)
			codePoint := (rune(byte1&0x1f) << 6) | rune(byte2&0x3f)
			result += string(codePoint)

		} else if byte1 >= 0xe0 && byte1 <= 0xef {
			// 3-byte sequence (1110xxxx 10xxxxxx 10xxxxxx)
			if i+2 >= len(arr) {
				continue
			}
			byte2 := arr[i+1]
			byte3 := arr[i+2]
			skip = 2

			// Logical Flaw 2: NO VALIDATION on byte2 or byte3
			codePoint := (rune(byte1&0x0f) << 12) |
				(rune(byte2&0x3f) << 6) |
				rune(byte3&0x3f)
			result += string(codePoint)

		} else if byte1 >= 0xf0 && byte1 <= 0xf7 {
			// 4-byte sequence (11110xxx 10xxxxxx 10xxxxxx 10xxxxxx)
			if i+3 >= len(arr) {
				continue
			}
			byte2 := arr[i+1]
			byte3 := arr[i+2]
			byte4 := arr[i+3]
			skip = 3

			// Logical Flaw 2: NO VALIDATION on continuation bytes
			codePoint := (rune(byte1&0x07) << 18) |
				(rune(byte2&0x3f) << 12) |
				(rune(byte3&0x3f) << 6) |
				rune(byte4&0x3f)

			// Go handles the conversion of a high code point (rune) to a
			// UTF-16 surrogate pair string (as required by the TS logic)
			// automatically when you cast the rune to a string.
			result += string(codePoint)

		}
		// Logical Flaw 3: Invalid start bytes (0x80-0xBF and 0xF8-0xFF)
		// fall through here and are SILENTLY SKIPPED, matching the TS behavior.
	}

	return result
}
