package model

import "bytes"

// RewriteHistory renames the model in a .his file. Every line starts with
// the model name followed by a comma; lines whose first field equals oldName
// (case-insensitive) get newName, everything else (including the line ends,
// CRLF or LF, and every other byte) is kept untouched. It returns the new
// content and the number of lines changed.
func RewriteHistory(data []byte, oldName, newName string) ([]byte, int) {
	var out bytes.Buffer
	out.Grow(len(data) + 16)
	changed := 0
	for len(data) > 0 {
		var line []byte
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			line, data = data[:i+1], data[i+1:]
		} else {
			line, data = data, nil
		}
		if c := bytes.IndexByte(line, ','); c >= 0 && Equal(string(line[:c]), oldName) {
			out.WriteString(newName)
			out.Write(line[c:])
			changed++
			continue
		}
		out.Write(line)
	}
	return out.Bytes(), changed
}
