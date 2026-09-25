package gen

import (
	"regexp"
	"strings"
	"unicode"
)

// BaseName strips one trailing "Service" from a proto service name, keeping
// the whole name when nothing would remain.
func BaseName(service string) string {
	base := strings.TrimSuffix(service, "Service")
	if base == "" {
		return service
	}
	return base
}

// lowerFirst lowers the first rune. A name that does not change gets a
// leading underscore so a type and its var never share an identifier.
func lowerFirst(s string) string {
	r := []rune(s)
	if len(r) == 0 || !unicode.IsUpper(r[0]) {
		return "_" + s
	}
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

func upperFirst(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

var (
	snakeAcronym = regexp.MustCompile(`([A-Z]+)([A-Z][a-z])`)
	snakeLower   = regexp.MustCompile(`([a-z\d])([A-Z])`)
)

// SnakeCase converts a CamelCase rpc name the way grpc's Ruby plugin does:
// GetAPIKey becomes get_api_key.
func SnakeCase(s string) string {
	s = snakeAcronym.ReplaceAllString(s, `${1}_${2}`)
	s = snakeLower.ReplaceAllString(s, `${1}_${2}`)
	s = strings.ReplaceAll(s, "-", "_")
	return strings.ToLower(s)
}

// PascalCase joins the words of s, split on any non-alphanumeric rune, each
// with its first letter upper-cased.
func PascalCase(s string) string {
	var b strings.Builder
	up := true
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			up = true
			continue
		}
		if up {
			r = unicode.ToUpper(r)
			up = false
		}
		b.WriteRune(r)
	}
	return b.String()
}

// ScreamingSnake upper-cases s with every non-alphanumeric run replaced by
// one underscore.
func ScreamingSnake(s string) string {
	var b strings.Builder
	sep := false
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			sep = b.Len() > 0
			continue
		}
		if sep {
			b.WriteByte('_')
			sep = false
		}
		b.WriteRune(unicode.ToUpper(r))
	}
	return b.String()
}

// goIdent makes s a valid Go identifier by replacing every other rune with
// an underscore.
func goIdent(s string) string {
	var b strings.Builder
	for i, r := range s {
		switch {
		case unicode.IsLetter(r) || r == '_', unicode.IsDigit(r) && i > 0:
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

// splitGoPackage splits a go_package option into import path and package
// name. Without a ";" the name is the last path element.
func splitGoPackage(opt string) (importPath, name string) {
	if i := strings.IndexByte(opt, ';'); i >= 0 {
		return opt[:i], opt[i+1:]
	}
	name = opt
	if i := strings.LastIndexByte(opt, '/'); i >= 0 {
		name = opt[i+1:]
	}
	return opt, goIdent(name)
}

// goMessageName is the Go type name protoc-gen-go gives a message: the name
// relative to its package with "." replaced by "_".
func goMessageName(m MessageRef) string {
	rel := strings.TrimPrefix(m.FullName, m.Package+".")
	return strings.ReplaceAll(rel, ".", "_")
}

// rubyModule is the module protoc's Ruby generator puts a file's classes in:
// ruby_package when set, else each package segment with underscores removed
// and word starts upper-cased, joined by "::".
func rubyModule(pkg, rubyPackage string) string {
	if rubyPackage != "" {
		return rubyPackage
	}
	segs := strings.Split(pkg, ".")
	for i, s := range segs {
		segs[i] = PascalCase(s)
	}
	return strings.Join(segs, "::")
}

// rubyMessageName is the fully qualified Ruby class of a message.
func rubyMessageName(m MessageRef) string {
	mod := rubyModule(m.Package, m.RubyPackage)
	rel := strings.TrimPrefix(m.FullName, m.Package+".")
	parts := strings.Split(rel, ".")
	for i, p := range parts {
		parts[i] = upperFirst(p)
	}
	return mod + "::" + strings.Join(parts, "::")
}

// rubyRequirePath is the load path of the *_pb.rb file protoc writes for a
// proto file.
func rubyRequirePath(protoPath string) string {
	return strings.TrimSuffix(protoPath, ".proto") + "_pb"
}

func rubyString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, `#`, `\#`)
	return `"` + s + `"`
}
