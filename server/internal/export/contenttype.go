package export

import "strings"

// ContentTypeForFormat maps a quick-format name (json/xml/csv/txt) to its HTTP
// Content-Type. Unknown formats fall back to a binary stream. This is the
// single source of truth used by both the quick-format exporter and the
// template-render path (P2-7.e).
func ContentTypeForFormat(format string) string {
	switch strings.ToLower(format) {
	case "json":
		return "application/json"
	case "xml":
		return "application/xml"
	case "csv":
		return "text/csv; charset=utf-8"
	case "txt", "text":
		return "text/plain; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}

// ContentTypeForFilename infers a Content-Type from a filename's extension,
// used when a rendered template names its output file. Delegates to
// ContentTypeForFormat so the mapping lives in one place.
func ContentTypeForFilename(name string) string {
	ext := strings.ToLower(filepathExt(name))
	switch ext {
	case ".json":
		return ContentTypeForFormat("json")
	case ".xml":
		return ContentTypeForFormat("xml")
	case ".csv":
		return ContentTypeForFormat("csv")
	case ".txt":
		return ContentTypeForFormat("txt")
	default:
		return ContentTypeForFormat("")
	}
}
