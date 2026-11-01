// Package skillfmt implements the skill format v1: the canonical hash, the
// front matter reader, the schema check, the file-layout check and the offline
// signature check. It makes no network call.
package skillfmt

// Violation is one refusal: the named rule, where it was found, and a sentence.
type Violation struct {
	Rule    string
	Path    string
	Message string
}

// MaxSkillMDBytes is the largest SKILL.md a verifier reads: 64 KiB. A skill
// is text an agent reads whole; 64 KiB is about 16 000 tokens, over three
// times the 5 000 the Agent Skills guidance suggests and far above the 1 024
// characters of a description, yet small enough that a verifier's memory use
// does not depend on the input.
const MaxSkillMDBytes = 64 * 1024

// Rule names a refusal the verifier can give.
type Rule struct {
	ID      string
	Summary string
	// Example is true when examples/invalid/<lowercase id>/ holds a skill
	// refused by this rule alone. The signature rules have no skill to refuse;
	// a symbolic link cannot be committed portably, so its test builds one.
	Example bool
}

// Rule identifiers. The text of each rule is in spec/SKILL_FORMAT_V1.md, section 9.
const (
	RuleSkillMDMissing      = "SF-SKILLMD-MISSING"
	RuleFileScript          = "SF-FILE-SCRIPT"
	RuleFileUnlisted        = "SF-FILE-UNLISTED"
	RuleFileSymlink         = "SF-FILE-SYMLINK"
	RuleTextEncoding        = "SF-TEXT-ENCODING"
	RuleHeaderMalformed     = "SF-HEADER-MALFORMED"
	RuleFrontMatterMissing  = "SF-FRONTMATTER-MISSING"
	RuleFrontMatterSyntax   = "SF-FRONTMATTER-SYNTAX"
	RuleKeyUnknown          = "SF-KEY-UNKNOWN"
	RuleNameRequired        = "SF-NAME-REQUIRED"
	RuleNamePattern         = "SF-NAME-PATTERN"
	RuleNameLength          = "SF-NAME-LENGTH"
	RuleNameDirMismatch     = "SF-NAME-DIR-MISMATCH"
	RuleDescriptionRequired = "SF-DESCRIPTION-REQUIRED"
	RuleDescriptionLength   = "SF-DESCRIPTION-LENGTH"
	RuleFieldType           = "SF-FIELD-TYPE"
	RuleCompatibilityLength = "SF-COMPATIBILITY-LENGTH"
	RuleMetadataType        = "SF-METADATA-TYPE"
	RuleMetadataValueType   = "SF-METADATA-VALUE-TYPE"
	RuleSaphanOrigin        = "SF-SAPHAN-ORIGIN"
	RuleSaphanDefault       = "SF-SAPHAN-DEFAULT"
	RuleSignatureMalformed  = "SF-SIGNATURE-MALFORMED"
	RulePubkeyMalformed     = "SF-PUBKEY-MALFORMED"
	RuleSignatureInvalid    = "SF-SIGNATURE-INVALID"
	RuleSkillMDSize         = "SF-SKILLMD-SIZE"
	// RuleSchemaUnattributed marks a schema failure whose sub-schema carries no
	// x-rule. It is not a published rule; a test fails if it is ever reported.
	RuleSchemaUnattributed = "SF-SCHEMA"
)

// Rules is the closed list of rules. A test holds the examples, the schema
// annotations and the specification text to this list.
var Rules = []Rule{
	{RuleSkillMDMissing, "the directory has no SKILL.md", true},
	{RuleFileScript, "a scripts/ entry beside SKILL.md: a script never enters a skill", true},
	{RuleFileUnlisted, "a file or directory other than SKILL.md, references/ and assets/", true},
	{RuleFileSymlink, "a symbolic link anywhere in the skill", false},
	{RuleTextEncoding, "SKILL.md is not UTF-8 text with LF line ends and no byte order mark", true},
	{RuleSkillMDSize, "SKILL.md is larger than 65536 bytes", true},
	{RuleHeaderMalformed, "a decision-projection header that is not closed or not followed by one blank line", true},
	{RuleFrontMatterMissing, "SKILL.md does not open with a front matter block closed by ---", true},
	{RuleFrontMatterSyntax, "the front matter is outside the subset of YAML the format reads", true},
	{RuleKeyUnknown, "a top-level front matter key the format does not define", true},
	{RuleNameRequired, "name is absent", true},
	{RuleNamePattern, "name is not made of a-z, 0-9 and hyphen only", true},
	{RuleNameLength, "name is longer than 64 characters", true},
	{RuleNameDirMismatch, "name differs from the name of the skill directory", true},
	{RuleDescriptionRequired, "description is absent", true},
	{RuleDescriptionLength, "description is empty or longer than 1024 characters", true},
	{RuleFieldType, "name, description, license, compatibility or allowed-tools is not a string", true},
	{RuleCompatibilityLength, "compatibility is longer than 500 characters", true},
	{RuleMetadataType, "metadata is not a map", true},
	{RuleMetadataValueType, "a metadata value is not a string", true},
	{RuleSaphanOrigin, "saphan.origin is neither builtin nor project", true},
	{RuleSaphanDefault, "saphan.default is neither on nor off", true},
	{RuleSignatureMalformed, "the signature file does not decode to 64 bytes", false},
	{RulePubkeyMalformed, "the public key file does not decode to an ed25519 key", false},
	{RuleSignatureInvalid, "the signature does not verify over the canonical hash", false},
}
