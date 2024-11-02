// Generic data manipulation utilities.

package utils

import (
	//"crypto/tls"
	//"encoding/json"
	//"errors"
	//"fmt"
	"net"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	//"time"
	"unicode"
	//"unicode/utf8"

	"github.com/tinode/chat/server/auth"
	"github.com/tinode/chat/server/datamodel"
	//"github.com/tinode/chat/server/globals"
	//"github.com/tinode/chat/server/logs"
	//"github.com/tinode/chat/server/store"
	"github.com/tinode/chat/server/store/types"

	//"golang.org/x/crypto/acme/autocert"
)

// Tag with prefix:
// * prefix starts with an ASCII letter, contains ASCII letters, numbers, from 2 to 16 chars
// * tag body may contain Unicode letters and numbers, as well as the following symbols: +-.!?#@_
// Tag body can be up to maxTagLength (96) chars long.
var prefixedTagRegexp = regexp.MustCompile(`^([a-z]\w{1,15}):[-_+.!?#@\pL\pN]{1,96}$`)

// Generic tag: the same restrictions as tag body.
var tagRegexp = regexp.MustCompile(`^[-_+.!?#@\pL\pN]{1,96}$`)

const nullValue = "\u2421"

// Convert a list of IDs into ranges
func DelrangeDeserialize(in []types.Range) []datamodel.MsgDelRange {
	if len(in) == 0 {
		return nil
	}

	out := make([]datamodel.MsgDelRange, 0, len(in))
	for _, r := range in {
		out = append(out, datamodel.MsgDelRange{LowId: r.Low, HiId: r.Hi})
	}

	return out
}

// Trim whitespace, remove short/empty tags and duplicates, convert to lowercase, ensure
// the number of tags does not exceed the maximum.
/*func NormalizeTags(src []string) types.StringSlice {
	if src == nil {
		return nil
	}

	// Make sure the number of tags does not exceed the maximum.
	// Technically it may result in fewer tags than the maximum due to empty tags and
	// duplicates, but that's user's fault.
	if len(src) > globals.Globals.maxTagCount {
		src = src[:globals.Globals.maxTagCount]
	}

	// Trim whitespace and force to lowercase.
	for i := 0; i < len(src); i++ {
		src[i] = strings.ToLower(strings.TrimSpace(src[i]))
	}

	// Sort tags
	sort.Strings(src)

	// Remove short, invalid tags and de-dupe keeping the order. It may result in fewer tags than could have
	// been if length were enforced later, but that's client's fault.
	var prev string
	var dst []string
	for _, curr := range src {
		if isNullValue(curr) {
			// Return non-nil empty array
			return make([]string, 0, 1)
		}

		// Unicode handling
		ucurr := []rune(curr)

		// Enforce length in characters, not in bytes.
		if len(ucurr) < minTagLength || len(ucurr) > maxTagLength || curr == prev {
			continue
		}

		// Make sure the tag starts with a letter or a number.
		if !unicode.IsLetter(ucurr[0]) && !unicode.IsDigit(ucurr[0]) {
			continue
		}

		dst = append(dst, curr)
		prev = curr
	}

	return types.StringSlice(dst)
}*/

// stringSliceDelta extracts the slices of added and removed strings from two slices:
//
//	added :=  newSlice - (oldSlice & newSlice) -- present in new but missing in old
//	removed := oldSlice - (oldSlice & newSlice) -- present in old but missing in new
//	intersection := oldSlice & newSlice -- present in both old and new
func StringSliceDelta(rold, rnew []string) (added, removed, intersection []string) {
	if len(rold) == 0 && len(rnew) == 0 {
		return nil, nil, nil
	}
	if len(rold) == 0 {
		return rnew, nil, nil
	}
	if len(rnew) == 0 {
		return nil, rold, nil
	}

	sort.Strings(rold)
	sort.Strings(rnew)

	// Match old slice against the new slice and separate removed strings from added.
	o, n := 0, 0
	lold, lnew := len(rold), len(rnew)
	for o < lold || n < lnew {
		if o == lold || (n < lnew && rold[o] > rnew[n]) {
			// Present in new, missing in old: added
			added = append(added, rnew[n])
			n++

		} else if n == lnew || rold[o] < rnew[n] {
			// Present in old, missing in new: removed
			removed = append(removed, rold[o])
			o++

		} else {
			// present in both
			intersection = append(intersection, rold[o])
			if o < lold {
				o++
			}
			if n < lnew {
				n++
			}
		}
	}
	return added, removed, intersection
}

// restrictedTagsEqual checks if two sets of tags contain the same set of restricted tags:
// true - same, false - different.
func RestrictedTagsEqual(oldTags, newTags []string, namespaces map[string]bool) bool {
	rold := FilterRestrictedTags(oldTags, namespaces)
	rnew := FilterRestrictedTags(newTags, namespaces)

	if len(rold) != len(rnew) {
		return false
	}

	sort.Strings(rold)
	sort.Strings(rnew)

	// Match old tags against the new tags.
	for i := 0; i < len(rnew); i++ {
		if rold[i] != rnew[i] {
			return false
		}
	}

	return true
}

// Process credentials for correctness: remove duplicate and unknown methods.
// In case of duplicate methods only the first one satisfying valueRequired is kept.
// If valueRequired is true, keep only those where Value is non-empty.
/*func NormalizeCredentials(creds []datamodel.MsgCredClient, valueRequired bool) []datamodel.MsgCredClient {
	if len(creds) == 0 {
		return nil
	}

	index := make(map[string]*datamodel.MsgCredClient)
	for i := range creds {
		c := &creds[i]
		if _, ok := globals.Globals.validators[c.Method]; ok && (!valueRequired || c.Value != "") {
			index[c.Method] = c
		}
	}
	creds = make([]datamodel.MsgCredClient, 0, len(index))
	for _, c := range index {
		creds = append(creds, *index[c.Method])
	}
	return creds
}*/

// Get a string slice with methods of credentials.
func CredentialMethods(creds []datamodel.MsgCredClient) []string {
	out := make([]string, len(creds))
	for i := range creds {
		out[i] = creds[i].Method
	}
	return out
}

// Check if the interface contains a string with a single Unicode Del control character.
func IsNullValue(i any) bool {
	if str, ok := i.(string); ok {
		return str == nullValue
	}
	return false
}

// Helper function to select access mode for the given auth level
func SelectAccessMode(authLvl auth.Level, anonMode, authMode, rootMode types.AccessMode) types.AccessMode {
	switch authLvl {
	case auth.LevelNone:
		return types.ModeNone
	case auth.LevelAnon:
		return anonMode
	case auth.LevelAuth:
		return authMode
	case auth.LevelRoot:
		return rootMode
	default:
		return types.ModeNone
	}
}

// Get default modeWant for the given topic category
func GetDefaultAccess(cat types.TopicCat, authUser, isChan bool) types.AccessMode {
	if !authUser {
		return types.ModeNone
	}

	switch cat {
	case types.TopicCatP2P:
		return types.ModeCP2P
	case types.TopicCatFnd:
		return types.ModeNone
	case types.TopicCatGrp:
		if isChan {
			return types.ModeCChnWriter
		}
		return types.ModeCPublic
	case types.TopicCatMe:
		return types.ModeCSelf
	default:
		panic("Unknown topic category")
	}
}

// Parse topic access parameters
func ParseTopicAccess(acs *datamodel.MsgDefaultAcsMode, defAuth, defAnon types.AccessMode) (authMode, anonMode types.AccessMode,
	err error) {

	authMode, anonMode = defAuth, defAnon

	if acs.Auth != "" {
		err = authMode.UnmarshalText([]byte(acs.Auth))
	}
	if acs.Anon != "" {
		err = anonMode.UnmarshalText([]byte(acs.Anon))
	}

	return
}

// Parse one component of a semantic version string.
func parseVersionPart(vers string) int {
	end := strings.IndexFunc(vers, func(r rune) bool {
		return !unicode.IsDigit(r)
	})

	t := 0
	var err error
	if end > 0 {
		t, err = strconv.Atoi(vers[:end])
	} else if len(vers) > 0 {
		t, err = strconv.Atoi(vers)
	}
	if err != nil || t > 0x1fff || t <= 0 {
		return 0
	}
	return t
}

// Parses semantic version string in the following formats:
//
//	1.2, 1.2abc, 1.2.3, 1.2.3-abc, v0.12.34-rc5
//
// Unparceable values are replaced with zeros.
func ParseVersion(vers string) int {
	var major, minor, patch int
	// Maybe remove the optional "v" prefix.
	vers = strings.TrimPrefix(vers, "v")

	// We can handle 3 parts only.
	parts := strings.SplitN(vers, ".", 3)
	count := len(parts)
	if count > 0 {
		major = parseVersionPart(parts[0])
		if count > 1 {
			minor = parseVersionPart(parts[1])
			if count > 2 {
				patch = parseVersionPart(parts[2])
			}
		}
	}

	return (major << 16) | (minor << 8) | patch
}

// Version as a base-10 number. Used by monitoring.
func Base10Version(hex int) int64 {
	major := hex >> 16 & 0xFF
	minor := hex >> 8 & 0xFF
	trailer := hex & 0xFF
	return int64(major*10000 + minor*100 + trailer)
}

func versionToString(vers int) string {
	str := strconv.Itoa(vers>>16) + "." + strconv.Itoa((vers>>8)&0xff)
	if vers&0xff != 0 {
		str += "-" + strconv.Itoa(vers&0xff)
	}
	return str
}

// Tag handling

// Take a slice of tags, return a slice of restricted namespace tags contained in the input.
// Tags to filter, restricted namespaces to filter.
func FilterRestrictedTags(tags []string, namespaces map[string]bool) []string {
	var out []string
	if len(namespaces) == 0 {
		return out
	}

	for _, s := range tags {
		parts := prefixedTagRegexp.FindStringSubmatch(s)

		if len(parts) < 2 {
			continue
		}

		if namespaces[parts[1]] {
			out = append(out, s)
		}
	}

	return out
}


// Returns > 0 if v1 > v2; zero if equal; < 0 if v1 < v2
// Only Major and Minor parts are compared, the trailer is ignored.
func VersionCompare(v1, v2 int) int {
	return (v1 >> 8) - (v2 >> 8)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Truncate string if it's too long. Used in logging.
func TruncateStringIfTooLong(s string) string {
	if len(s) <= 1024 {
		return s
	}

	return s[:1024] + "..."
}

// Convert relative filepath to absolute.
func ToAbsolutePath(base, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Clean(filepath.Join(base, path))
}

// Detect platform from the UserAgent string.
func PlatformFromUA(ua string) string {
	ua = strings.ToLower(ua)
	switch {
	case strings.Contains(ua, "reactnative"):
		switch {
		case strings.Contains(ua, "iphone"),
			strings.Contains(ua, "ipad"):
			return "ios"
		case strings.Contains(ua, "android"):
			return "android"
		}
		return ""
	case strings.Contains(ua, "tinodejs"):
		return "web"
	case strings.Contains(ua, "tindroid"):
		return "android"
	case strings.Contains(ua, "tinodios"):
		return "ios"
	}
	return ""
}

/*func ParseTLSConfig(tlsEnabled bool, jsconfig json.RawMessage) (*tls.Config, error) {
	type tlsAutocertConfig struct {
		// Domains to support by autocert
		Domains []string `json:"domains"`
		// Name of directory where auto-certificates are cached, e.g. /etc/letsencrypt/live/your-domain-here
		CertCache string `json:"cache"`
		// Contact email for letsencrypt
		Email string `json:"email"`
	}

	type tlsConfig struct {
		// Flag enabling TLS
		Enabled bool `json:"enabled"`
		// Listen for connections on this address:port and redirect them to HTTPS port.
		RedirectHTTP string `json:"http_redirect"`
		// Enable Strict-Transport-Security by setting max_age > 0
		StrictMaxAge int `json:"strict_max_age"`
		// ACME autocert config, e.g. letsencrypt.org
		Autocert *tlsAutocertConfig `json:"autocert"`
		// If Autocert is not defined, provide file names of static certificate and key
		CertFile string `json:"cert_file"`
		KeyFile  string `json:"key_file"`
	}

	var config tlsConfig

	if jsconfig != nil {
		if err := json.Unmarshal(jsconfig, &config); err != nil {
			return nil, errors.New("http: failed to parse tls_config: " + err.Error() + "(" + string(jsconfig) + ")")
		}
	}

	if !tlsEnabled && !config.Enabled {
		return nil, nil
	}

	if config.StrictMaxAge > 0 {
		globals.Globals.tlsStrictMaxAge = strconv.Itoa(config.StrictMaxAge)
	}

	globals.Globals.tlsRedirectHTTP = config.RedirectHTTP

	// If autocert is provided, use it.
	if config.Autocert != nil {
		certManager := autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(config.Autocert.Domains...),
			Cache:      autocert.DirCache(config.Autocert.CertCache),
			Email:      config.Autocert.Email,
		}
		return certManager.TLSConfig(), nil
	}

	// Otherwise try to use static keys.
	cert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
	if err != nil {
		return nil, err
	}

	return &tls.Config{Certificates: []tls.Certificate{cert}}, nil
}*/

// Merge source interface{} into destination interface.
// If values are maps,deep-merge them. Otherwise shallow-copy.
// Returns dst, true if the dst value was changed.
func MergeInterfaces(dst, src any) (any, bool) {
	var changed bool

	if src == nil {
		return dst, changed
	}

	vsrc := reflect.ValueOf(src)
	switch vsrc.Kind() {
	case reflect.Map:
		if xsrc, ok := src.(map[string]any); ok {
			xdst, _ := dst.(map[string]any)
			dst, changed = mergeMaps(xdst, xsrc)
		} else {
			changed = true
			dst = src
		}
	case reflect.String:
		if vsrc.String() == nullValue {
			changed = dst != nil
			dst = nil
		} else {
			changed = true
			dst = src
		}
	default:
		changed = true
		dst = src
	}
	return dst, changed
}

// Deep copy maps.
func mergeMaps(dst, src map[string]any) (map[string]any, bool) {
	var changed bool

	if len(src) == 0 {
		return dst, changed
	}

	if dst == nil {
		dst = make(map[string]any)
	}

	for key, val := range src {
		xval := reflect.ValueOf(val)
		switch xval.Kind() {
		case reflect.Map:
			if xsrc, _ := val.(map[string]any); xsrc != nil {
				// Deep-copy map[string]interface{}
				xdst, _ := dst[key].(map[string]any)
				var lchange bool
				dst[key], lchange = mergeMaps(xdst, xsrc)
				changed = changed || lchange
			} else if val != nil {
				// The map is shallow-copied if it's not of the type map[string]interface{}
				dst[key] = val
				changed = true
			}
		case reflect.String:
			changed = true
			if xval.String() == nullValue {
				delete(dst, key)
			} else if val != nil {
				dst[key] = val
			}
		default:
			if val != nil {
				dst[key] = val
				changed = true
			}
		}
	}

	return dst, changed
}

// netListener creates net.Listener for tcp and unix domains:
// if addr is in the form "unix:/run/tinode.sock" it's a unix socket, otherwise TCP host:port.
func NetListener(addr string) (net.Listener, error) {
	addrParts := strings.SplitN(addr, ":", 2)
	if len(addrParts) == 2 && addrParts[0] == "unix" {
		return net.Listen("unix", addrParts[1])
	}
	return net.Listen("tcp", addr)
}

// Check if specified address is a unix socket like "unix:/run/tinode.sock".
func IsUnixAddr(addr string) bool {
	addrParts := strings.SplitN(addr, ":", 2)
	return len(addrParts) == 2 && addrParts[0] == "unix"
}

var privateIPBlocks []*net.IPNet

func IsRoutableIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return false
	}

	if privateIPBlocks == nil {
		for _, cidr := range []string{
			"10.0.0.0/8",     // RFC1918
			"172.16.0.0/12",  // RFC1918
			"192.168.0.0/16", // RFC1918
			"fc00::/7",       // RFC4193, IPv6 unique local addr
		} {
			_, block, _ := net.ParseCIDR(cidr)
			privateIPBlocks = append(privateIPBlocks, block)
		}
	}

	for _, block := range privateIPBlocks {
		if block.Contains(ip) {
			return false
		}
	}
	return true
}
