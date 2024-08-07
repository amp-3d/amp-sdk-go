package tag

import (
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/amp-3d/amp-sdk-go/stdlib/bufs"
)

var (
	sTagSeparator = regexp.MustCompile(`[/\\\.+:\s\~]+`) // Which delimiters?  'significant" delimiters.
)

const (
	CanonicWithRune = '.'
	CanonicHideRune = '~'
)

func (id ID) AppendAsOctals(enc []OctalDigit) []OctalDigit {
	remain := ID{id[0], id[1], id[2]}
	digits := 0

	for bitsRemain := 192; bitsRemain > 0; bitsRemain -= 3 {
		digit := OctalDigit(remain[2] & 0x7)
		enc = append(enc, digit)
		enc[digits] = digit
		remain[2] = (remain[2] >> 3) | (remain[1] << 61)
		remain[1] = (remain[1] >> 3) | (remain[0] << 61)
		remain[0] = (remain[0] >> 3)
		digits++
	}
	return enc
}

func (id ID) FormAsciiBadge() string {
	encBuf := make([]OctalDigit, 0, 64)
	enc := id.AppendAsOctals(encBuf)

	str := append([]byte{}, gBadge6424.Template...)
	for i, di := range gBadge6424.Dots {
		val := CanonicAsciiAlphabet[0]
		if i < len(enc) {
			val = CanonicAsciiAlphabet[enc[i]]
		}

		// Set the dot value where it belongs in the cloned template
		str[di.CharIndex] = byte(val)
	}

	return string(str)
}

// Composite tag expression syntax
//
//	tag.Spec := "[{utf8_tag_literal}[.:/\\w]*]*"
//
// Note how a tag spec with no delimeters is a pure element type descriptor (and AttrSpecID == ElemSpecID)
type Spec struct {
	ID      ID
	Canonic string
	Tags    []Literal
}

func (spec Spec) CanonicString() string {
	if spec.Canonic == "" {
		b := strings.Builder{}
		for _, tag := range spec.Tags {
			if b.Len() > 0 {
				b.WriteRune(CanonicWithRune)
			}
			b.WriteString(tag.Token)
		}
		spec.Canonic = b.String()
	}
	return spec.Canonic
}

// LeafTags splits the tag spec the given number of tags for the right.
// E.g. LeafTags(2) on "a.b.c.d.ee" yields ("a.b.c", "d.ee")
func (spec Spec) LeafTags(n int) (string, string) {
	if n <= 0 {
		return spec.Canonic, ""
	}

	expr := spec.Canonic
	R := len(expr)
	for p := R - 1; p >= 0; p-- {
		switch expr[p] {
		case CanonicWithRune:
			fallthrough
		case CanonicHideRune:
			n--
			if n <= 0 {
				return expr[:p], expr[p+1:]
			}
		}
	}
	return "", expr
}

// A tag.Spec produces a tag.ID such that each tag.ID is unique and is independent of its component tag literals.
//
//	e.g. "a.b.cc" == "b.a.cc" == "a.cc.b" != "a.cC.b"
func (spec Spec) With(subTags string) Spec {
	newSpec := Spec{
		ID:   spec.ID,
		Tags: make([]Literal, 0, 8),
	}

	canonic := make([]byte, 0, len(spec.Canonic)+len(subTags))
	canonic = append(canonic, spec.Canonic...)
	tags := sTagSeparator.Split(subTags, 37)
	if len(tags) > 0 {

		for _, ti := range tags {
			if ti == "" { // empty tokens are no-ops
				continue
			}
			if len(canonic) > 0 {
				canonic = append(canonic, CanonicWithRune)
			}
			canonic = append(canonic, []byte(ti)...)
			literal := Literal{
				ID:    FromLiteral([]byte(ti)),
				Token: ti,
			}
			newSpec.Tags = append(newSpec.Tags, literal)
			newSpec.ID = newSpec.ID.With(literal.ID)
		}
		newSpec.Canonic = string(canonic)
	}

	return newSpec
}

// ID is a signed 24 byte UTC time index in big endian form, with 6 bytes of signed seconds and 10 bytes of fractional precision.
// This means there are 47 bits dedicated for whole seconds => +/- 4.4 million years
//
// This also means (ID[0] >> 16) yields a standard 64-bit Unix UTC time.
type ID [3]uint64

const (
	NanosecStep = uint64(0x44B82FA1C) // 1<<64 div 1e9 -- reflects Go's single nanosecond resolution spread over a 64 bits
	EntropyMask = uint64(0x3FFFFFFFF) // entropy bit mask for ID[1] -- slightly smaller than 1 ns resolution
)

func FromLiteral(tagLiteral []byte) ID {
	var hashBuf [16]byte

	hasher := md5.New()
	hasher.Write(tagLiteral)
	hash := hasher.Sum(hashBuf[:0])

	return ID{
		0, // tag / token ops don't have to affect hash[0]
		binary.LittleEndian.Uint64(hash[0:]),
		binary.LittleEndian.Uint64(hash[8:]),
	}
}

func FromString(unclean string) ID {
	tagLiteral := sTagSeparator.ReplaceAll([]byte(unclean), nil)
	return FromLiteral(tagLiteral)
}

func FromToken(literal string) ID {
	return FromLiteral([]byte(literal))
}

func FromTime(t time.Time, addEntropy bool) ID {
	ns_b10 := uint64(t.Nanosecond())
	ns_f64 := ns_b10 * NanosecStep // map 0..999999999 to 0..(2^64-1)

	t_00_06 := uint64(t.Unix()) << 16
	t_06_08 := ns_f64 >> 48
	t_08_15 := ns_f64 << 16
	tag := ID{
		t_00_06 | uint64(t_06_08),
		t_08_15,
		0,
	}

	if addEntropy {
		gTagSeed = 377377733*ns_f64 ^ gTagSeed
		tag[1] ^= gTagSeed & EntropyMask
		tag[2] ^= gTagSeed * ns_f64
	}

	return tag
}

func Join(prefixTags, suffixTags string) string {
	if prefixTags == "" {
		return suffixTags
	}
	if suffixTags == "" {
		return prefixTags
	}
	if (prefixTags[len(prefixTags)-1] != '.') && (suffixTags[0] != '.') {
		return prefixTags + "." + suffixTags
	}
	if (prefixTags[len(prefixTags)-1] == '.') && (suffixTags[0] == '.') {
		return prefixTags + suffixTags[1:]
	}
	return prefixTags + suffixTags
}

func (tag Literal) String() string {
	b := strings.Builder{}
	b.Grow(len(tag.Token) + 10)
	if tag.Token != "" {
		fmt.Fprintf(&b, "%q", tag.Token)
	} else {
		b.WriteString(tag.ID.Base32())
	}
	return b.String()
}

// Returns the current time as a tag.ID, statistically guaranteed to be unique even when called in rapid succession.
func Now() ID {
	return FromTime(time.Now(), true)
}

func (id ID) IsNil() bool {
	return id[0] == 0 && id[1] == 0 && id[2] == 0
}

func (id ID) IsSet() bool {
	return id[0] != 0 || id[1] != 0 || id[2] != 0
}

func (id ID) IsWildcard() bool {
	return id[0] == 0x1 && id[1] == 0x1 && id[2] == 0x1
}

// This operator is commutative and associative, and is used to generate a new ID from two existing ones.
// Since this is commutative, it is reversible, and means tag literals are order independent.
func (id ID) With(other ID) ID {
	return ID{
		id[0] + other[0],
		id[1] + other[1], // overflow is normal
		id[2] + other[2], // overflow is normal
	}
}

// Entangles this ID with another, producing a new ID -- non-commutative.
func (id ID) Then(other ID) ID {
	return ID{
		id[0] - other[0],
		id[1] - other[1], // overflow is normal
		id[2] - other[2], // overflow is normal
	}
}

func (id ID) WithToken(tagToken string) ID {
	return id.WithLiteral([]byte(tagToken))
}

func (id ID) WithLiteral(tagLiteral []byte) ID {
	return id.With(FromLiteral(tagLiteral))
}

func (tag ID) String() string {
	return tag.Base32()
}

func (tag ID) CompareTo(oth ID) int {
	if tag[0] < oth[0] {
		return -1
	}
	if tag[0] > oth[0] {
		return 1
	}
	if tag[1] < oth[1] {
		return -1
	}
	if tag[1] > oth[1] {
		return 1
	}
	if tag[2] < oth[2] {
		return -1
	}
	if tag[2] > oth[2] {
		return 1
	}
	return 0
}

func (tag ID) Add(oth ID) ID {
	var out ID
	var carry uint64

	sum := tag[2] + oth[2]
	out[2] = sum
	if sum < tag[2] || sum < oth[2] {
		carry = 1
	}

	// no carry for tag[0]
	out[1] = tag[1] + oth[1] + carry
	out[0] = tag[0] + oth[0]
	return out
}

func (tag ID) Sub(oth ID) ID {
	var out ID
	var borrow uint64

	dif := tag[2] - oth[2]
	out[2] = dif
	if tag[2] < oth[2] || dif > tag[2] {
		borrow = 1
	}
	// no borrow for tag[0] -- by convention, first bytes are a signed UTC seconds value with 16 bits of fixed seconds precision
	out[1] = tag[1] - oth[1] - borrow
	out[0] = tag[0] - oth[0]
	return out
}

// Returns Unix UTC time in milliseconds
func (tag ID) UnixMilli() int64 {
	return int64(tag[0]*1000) >> 16
}

// Returns Unix UTC time in seconds
func (tag ID) Unix() int64 {
	return int64(tag[0]) >> 16
}

// Returns this tag.ID in canonic Base32 form
func (tag ID) Base32() string {
	var buf [25]byte // (25 * 8) % 5 == 0
	binary := tag.AppendTo(buf[:1])
	str := bufs.Base32Encoding.EncodeToString(binary)
	return str
}

func (tag ID) Base16() string {
	buf := make([]byte, 0, 48)
	tagBytes := tag.AppendTo(buf)
	str := hex.EncodeToString(tagBytes)
	return str
}

// Base32Suffix returns the last few digits of this TID in string form (for easy reading, logs, etc)
func (tag ID) Base32Suffix() string {
	const lcm_bits = 40 // divisible by 5 (bits) and 8 (bytes).
	const lcm_bytes = lcm_bits / 8

	var suffix [lcm_bytes]byte
	for i := uint(0); i < lcm_bytes; i++ {
		shift := uint(8 * (lcm_bytes - 1 - i))
		suffix[i] = byte(tag[2] >> shift)
	}
	base32 := bufs.Base32Encoding.EncodeToString(suffix[:])
	return base32
}

// Base16Suffix returns the last few digits of this TID in string form (for easy reading, logs, etc)
func (tag ID) Base16Suffix() string {
	const nibbles = 7
	const HexChars = "0123456789abcdef"

	var suffix [nibbles]byte
	for i := uint(0); i < nibbles; i++ {
		shift := uint(4 * (nibbles - 1 - i))
		hex := byte(tag[2]>>shift) & 0xF
		suffix[i] = HexChars[hex]
	}
	base16 := string(suffix[:])
	return base16
}

// CopyNext copies the given TID and increments it by 1, typically useful for seeking the next entry after a given one.
func (tag ID) Xor(other ID) ID {
	return ID{
		tag[0] ^ other[0],
		tag[1] ^ other[1],
		tag[2] ^ other[2],
	}
}

// Forms an amp.UID explicitly from two uint64 values.
func IntsToID(x0 int64, x1, x2 uint64) ID {
	return ID{
		uint64(x0),
		x1,
		x2,
	}
}

type Key [24]byte

var gTagSeed = uint64(0x3773000000003773)

var (
	Nil = ID{}
)

func FromBytes(in []byte) (tag ID, err error) {
	var buf [24]byte
	startAt := max(0, 24-len(in))
	copy(buf[startAt:], in)

	tag[0] = binary.BigEndian.Uint64(buf[0:8])
	tag[1] = binary.BigEndian.Uint64(buf[8:16])
	tag[2] = binary.BigEndian.Uint64(buf[16:24])
	return tag, nil
}

func (tag ID) AppendTo(dst []byte) []byte {
	dst = binary.BigEndian.AppendUint64(dst, tag[0])
	dst = binary.BigEndian.AppendUint64(dst, tag[1])
	dst = binary.BigEndian.AppendUint64(dst, tag[2])
	return dst
}

func (tag ID) ToLSM() Key {
	var key Key
	tag.Put24(key[:])
	return key
}

func From24(lsm []byte) (id ID) {
	id[0] = uint64(FromZigZag(binary.BigEndian.Uint64(lsm[0:])))
	id[1] = binary.BigEndian.Uint64(lsm[8:])
	id[2] = binary.BigEndian.Uint64(lsm[16:])
	return
}

func From16(lsm []byte) (id ID) {
	id[0] = 0
	id[1] = binary.BigEndian.Uint64(lsm[0:])
	id[2] = binary.BigEndian.Uint64(lsm[8:])
	return
}

func (tag ID) Put24(dst []byte) {
	binary.BigEndian.PutUint64(dst[0:], ToZigZag(int64(tag[0])))
	binary.BigEndian.PutUint64(dst[8:], tag[1])
	binary.BigEndian.PutUint64(dst[16:], tag[2])
}

func (tag ID) Put16(dst []byte) {
	binary.BigEndian.PutUint64(dst[0:], tag[1])
	binary.BigEndian.PutUint64(dst[8:], tag[2])
}

// Encodes a int64 to a zig-zag uint64
func ToZigZag(x int64) uint64 {
	ux := uint64(x) << 1
	if x < 0 {
		ux = ^ux
	}
	return ux
}

// Decodes a zig-zag uint64 to a int64
func FromZigZag(ux uint64) int64 {
	x := ux >> 1
	if ux&1 != 0 {
		x = ^x
	}
	return int64(x)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

/*
func (attrTag TagSpecID) PinLevel() int { // TODO: remove PinLevel!?  Just stuff it in the HaloTag spec and MD5 that.
	return int(attrTag[0] >> 61)
}

const (
	PinLevelBits = 3
	PinLevelMax  = (1 << PinLevelBits) - 1

	pinLevelMask  = uint64(PinLevelMax) << 61
	pinLevelShift = 64 - PinLevelBits
)

func (attrID *TagSpecID) ApplyPinLevel(pinLevel int) {
	attrID[0] &^= pinLevelMask
	attrID[0] |= uint64(pinLevel) << pinLevelShift
}

func (attrID *TagSpecID) IsNil() bool {
	return attrID[0] == 0 && attrID[1] == 0
}


var attrLexer = lexer.MustSimple([]lexer.SimpleRule{
	{Name: "Number", Pattern: `(?:\d*\.)?\d+`},
	{Name: "Ident", Pattern: `[a-zA-Z][-._\w]*`},
	{Name: "Punct", Pattern: `[[:/]|]`},
	{Name: "Whitespace", Pattern: `[ \t\n\r]+`},
	//{"Comment", `(?:#|//)[^\n]*\n?`},
	//{"Number", `(?:\d*\.)?\d+`},
	//{Name: "Punct", Pattern: `[[!@#$%^&*()+_={}\|:;"'<,>.?/]|]`},
})

type TagSpecExpr struct {
	PinLevel   int    `( @Number ":" )?`
	SeriesSpec string `( "[" (@Ident)? "]" )?`
	ElemType   string ` @Ident `
	AttrName   string `( ":" @Ident )?`
	AsCanonic  string
}

var tagSpecParser = participle.MustBuild[TagSpecExpr](
	participle.Lexer(attrLexer),
	participle.Elide("Whitespace"),
	//, participle.UseLookahead(2))
)

// ParseUID decodes s into a UID or returns an error.  Accepted forms:
//   - xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
//   - urn:uuid:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
//   - {xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx}
//   - xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx.
func ParseUUID(s string) (Tag, error) {
	uidBytes, err := uuid.Parse(s)
	if err != nil {
		return Tag{}, err
	}
	return Bin24ToTag(uidBytes[:])
}

// MustParseUID decodes s into a UID or panics -- see ParseUID().
func MustParseUID(s string) UID {
	uidBytes := uuid.MustParse(s)
	uid, _ := BytesToUID(uidBytes[:])
	return uid
}

// String returns the string form of uid: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx or "" if uuid is zero.
func (uid UID) ToUUID() (uuid uuid.UUID) {
	binary.BigEndian.PutUint64(uuid[:8], uid[0])
	binary.BigEndian.PutUint64(uuid[8:], uid[1])
	return uuid
}

// String returns the string form of uid: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx or "" if uuid is zero.
func (uid UID) String() string {
	return uid.ToUUID().String()
}

// Appends the base 32 ASCII encoding of this TID to the given buffer
func (tag TID) AppendAsBase32(in []byte) []byte {
	encLen := bufs.Base32Encoding.EncodedLen(len(ID))
	needed := len(in) + encLen
	buf := in
	if needed > cap(buf) {
		buf = make([]byte, (needed+0x100)&^0xFF)
		buf = append(buf[:0], in...)
	}
	buf = buf[:needed]
	bufs.Base32Encoding.Encode(buf[len(in):needed], ID)
	return buf
}

// SetTimeAndHash writes the given timestamp and the right-most part of inSig into this TID.
//
// See comments for TIDBinaryLen
func (tag TID) SetTimeAndHash(time UTC16, hash []byte) {
	tag.SetUTC(time)
	tag.SetHash(hash)
}

// SetHash sets the sig/hash portion of this ID
func (tag TID) SetHash(hash []byte) {
	const TIDHashSz = int(Const_TIDBinaryLen - 8)
	pos := len(hash) - TIDHashSz
	if pos >= 0 {
		copy(tag[TIDHashSz:], hash[pos:])
	} else {
		for i := 8; i < int(Const_TIDBinaryLen); i++ {
			tag[i] = hash[i]
		}
	}
}

// SelectEarlier looks in inTime a chooses whichever is earlier.
//
// If t is later than the time embedded in this TID, then this function has no effect and returns false.
//
// If t is earlier, then this TID is initialized to t (and the rest zeroed out) and returns true.
func (tag TID) SelectEarlier(t UTC16) bool {

	TIDt := tag.ExtractUTC()

	// Timestamp of 0 is reserved and should only reflect an invalid/uninitialized TID.
	if t < 0 {
		t = 0
	}

	if t < TIDt || t == 0 {
		tag.SetUTC(t)
		for i := 8; i < len(ID); i++ {
			tag[i] = 0
		}
		return true
	}

	return false
}


// TID identifies a Tx (or Cell) by secure hash ID.
type TxID struct {
	UTC16 UTC16
	Hash1 uint64
	Hash2 uint64
	Hash3 uint64
}

// Base32 returns this TID in Base32 form.
func (tag *TxID) Base32() string {
	var bin [TIDBinaryLen]byte
	binStr := tag.AppendAsBinary(bin[:0])
	return bufs.Base32Encoding.EncodeToString(binStr)
}

// Appends the base 32 ASCII encoding of this TID to the given buffer
func (tag *TxID) AppendAsBase32(io []byte) []byte {
	L := len(io)

	needed := L + TIDStringLen
	dst := io
	if needed > cap(dst) {
		dst = make([]byte, (needed+0x100)&^0xFF)
		dst = append(dst[:0], io...)
	}
	dst = dst[:needed]

	var bin [TIDBinaryLen]byte
	binStr := tag.AppendAsBinary(bin[:0])

	bufs.Base32Encoding.Encode(dst[L:needed], binStr)
	return dst
}

// Appends the base 32 ASCII encoding of this TID to the given buffer
func (tag *TxID) AppendAsBinary(io []byte) []byte {
	L := len(io)
	needed := L + TIDBinaryLen
	dst := io
	if needed > cap(dst) {
		dst = make([]byte, needed)
		dst = append(dst[:0], io...)
	}
	dst = dst[:needed]

	binary.BigEndian.PutUint64(dst[L+0:L+8], uint64(tag.UTC16))
	binary.BigEndian.PutUint64(dst[L+8:L+16], tag.Hash1)
	binary.BigEndian.PutUint64(dst[L+16:L+24], tag.Hash2)
	binary.BigEndian.PutUint64(dst[L+24:L+32], tag.Hash3)
	return dst
}


// ReadCell loads a cell with the given URI having the inferred schema (built from its fields using reflection).
// The URI is scoped into the user's home space and AppID.
func ReadCell(ctx AppContext, subKey string, schema *AttrSchema, dstStruct any) error {

	dst := reflect.Indirect(reflect.ValueOf(dstStruct))
	switch dst.Kind() {
	case reflect.Pointer:
		dst = dst.Elem()
	case reflect.Struct:
	default:
		return ErrCode_ExportErr.Errorf("expected struct, got %v", dst.Kind())
	}

	var keyBuf [128]byte
	cellKey := append(append(keyBuf[:0], []byte(ctx.StateScope())...), []byte(subKey)...)

	msgs := make([]*Msg, 0, len(schema.Attrs))
	err := ctx.LoginInfo().HomePlanet().ReadCell(cellKey, schema, func(msg *Msg) {
		switch msg.Op {
		case MsgOp_PushAttr:
			msgs = append(msgs, msg)
		}
	})
	if err != nil {
		return err
	}

	numFields := dst.NumField()
	valType := dst.Type()

	for fi := 0; fi < numFields; fi++ {
		field := valType.Field(fi)
		for _, ai := range schema.Attrs {
			if ai.TypedName == field.Name {
				for _, msg := range msgs {
					if msg.TagSpecID == ai.TagSpecID {
						msg.LoadVal(dst.Field(fi).Addr().Interface())
						goto nextField
					}
				}
			}
		}
	nextField:
	}
	return err
}

// WriteCell is the write analog of ReadCell.
func WriteCell(ctx AppContext, subKey string, schema *AttrSchema, srcStruct any) error {

	src := reflect.Indirect(reflect.ValueOf(srcStruct))
	switch src.Kind() {
	case reflect.Pointer:
		src = src.Elem()
	case reflect.Struct:
	default:
		return ErrCode_ExportErr.Errorf("expected struct, got %v", src.Kind())
	}

	{
		tx := NewMsgBatch()
		msg := tx.AddMsg()
		msg.Op = MsgOp_UpsertCell
		msg.ValType = ValType_SchemaID.Ord()
		msg.ValInt = int64(schema.SchemaID)
		msg.ValBuf = append(append(msg.ValBuf[:0], []byte(ctx.StateScope())...), []byte(subKey)...)

		numFields := src.NumField()
		valType := src.Type()

		for _, attr := range schema.Attrs {
			msg := tx.AddMsg()
			msg.Op = MsgOp_PushAttr
			msg.TagSpecID = attr.TagSpecID
			for i := 0; i < numFields; i++ {
				if valType.Field(i).Name == attr.TypedName {
					msg.setVal(src.Field(i).Interface())
					break
				}
			}
			if msg.ValType == ValType_nil.Ord() {
				panic("missing field")
			}
		}

		msg = tx.AddMsg()
		msg.Op = MsgOp_Commit

		if err := ctx.LoginInfo().HomePlanet().PushTx(tx); err != nil {
			return err
		}
	}

	return nil
}


*/
