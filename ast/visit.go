package ast

// Visitor traverses a Hurl AST depth-first, in source order (it is what the
// text and HTML formatters are built on). It mirrors Rust's visit.rs.
//
// Methods for composite nodes return a bool: true means "walk this node's
// children with the default traversal" (the corresponding Walk* function),
// false means the visitor handled the children itself or wants to skip them.
// To act both before and after a node's children, call the Walk* function
// yourself and return false:
//
//	func (f *htmlFormatter) VisitEntry(e *ast.Entry) bool {
//		f.open("hurl-entry")
//		ast.WalkEntry(f, e)
//		f.close()
//		return false
//	}
//
// Methods for leaf nodes return nothing. Embed BaseVisitor to get the
// default behaviour for every method you don't override. Start a traversal
// with Walk.
type Visitor interface {
	VisitAssert(a *Assert) bool
	VisitBase64(b *Base64) bool
	VisitBase64Value(value []byte, source string)
	VisitBody(b *Body) bool
	VisitBool(value bool)
	VisitBoolOption(o BooleanOption) bool
	VisitCapture(c *Capture) bool
	VisitCookie(c *Cookie) bool
	VisitCookiePath(p *CookiePath)
	VisitComment(c *Comment)
	VisitCount(c Count) bool
	VisitCountOption(o CountOption) bool
	VisitDuration(d *Duration) bool
	VisitDurationOption(o DurationOption) bool
	VisitDurationUnit(u DurationUnit)
	VisitEntry(e *Entry) bool
	VisitEntryOption(o *EntryOption) bool
	VisitFile(f *File) bool
	VisitFilenameParam(p *FilenameParam) bool
	VisitFilenameValue(v *FilenameValue) bool
	VisitFilename(filename *Template)
	VisitFilter(f *Filter) bool
	VisitFilterKind(kind FilterValue)
	VisitHeader(h *KeyValue) bool
	VisitHex(h *Hex) bool
	VisitHexValue(value []byte, source string)
	VisitHurlFile(f *HurlFile) bool
	VisitIntegerValue(n IntegerValue) bool
	VisitI64(n int64)
	VisitJSONBody(json JSONValue)
	VisitKeyValue(kv *KeyValue) bool
	VisitLineTerminator(lt *LineTerminator) bool
	VisitLiteral(lit string)
	VisitMethod(m Method)
	VisitMultilineString(s *MultilineString)
	VisitNaturalOption(o NaturalOption) bool
	VisitNot(identifier string)
	VisitNull(identifier string)
	VisitNumber(n Number)
	VisitPlaceholder(p *Placeholder)
	VisitPredicate(p *Predicate) bool
	VisitPredicateKind(kind PredicateFuncValue)
	VisitPredicateValue(v PredicateValue) bool
	VisitQuery(q *Query) bool
	VisitQueryKind(kind QueryValue)
	VisitRequest(r *Request) bool
	VisitResponse(r *Response) bool
	VisitRegex(r *Regex)
	VisitSection(s *Section) bool
	VisitStatus(s StatusValue)
	VisitString(value string)
	VisitSectionHeader(name string)
	VisitSectionValue(v SectionValue) bool
	VisitTemplate(t *Template)
	VisitURL(url *Template)
	VisitU64(n *U64)
	VisitUsize(n int)
	VisitVariableDef(d *VariableDefinition) bool
	VisitVariableName(name string)
	VisitVariableValue(v VariableValue) bool
	VisitVerbosityOption(v VerbosityOption) bool
	VisitVersion(v VersionValue)
	VisitXMLBody(xml string)
	VisitWhitespace(ws *Whitespace)
}

// BaseVisitor implements every Visitor method with the default behaviour:
// composite nodes are walked, leaves are ignored. Embed it in your visitor.
type BaseVisitor struct{}

func (BaseVisitor) VisitAssert(*Assert) bool                  { return true }
func (BaseVisitor) VisitBase64(*Base64) bool                  { return true }
func (BaseVisitor) VisitBase64Value([]byte, string)           {}
func (BaseVisitor) VisitBody(*Body) bool                      { return true }
func (BaseVisitor) VisitBool(bool)                            {}
func (BaseVisitor) VisitBoolOption(BooleanOption) bool        { return true }
func (BaseVisitor) VisitCapture(*Capture) bool                { return true }
func (BaseVisitor) VisitCookie(*Cookie) bool                  { return true }
func (BaseVisitor) VisitCookiePath(*CookiePath)               {}
func (BaseVisitor) VisitComment(*Comment)                     {}
func (BaseVisitor) VisitCount(Count) bool                     { return true }
func (BaseVisitor) VisitCountOption(CountOption) bool         { return true }
func (BaseVisitor) VisitDuration(*Duration) bool              { return true }
func (BaseVisitor) VisitDurationOption(DurationOption) bool   { return true }
func (BaseVisitor) VisitDurationUnit(DurationUnit)            {}
func (BaseVisitor) VisitEntry(*Entry) bool                    { return true }
func (BaseVisitor) VisitEntryOption(*EntryOption) bool        { return true }
func (BaseVisitor) VisitFile(*File) bool                      { return true }
func (BaseVisitor) VisitFilenameParam(*FilenameParam) bool    { return true }
func (BaseVisitor) VisitFilenameValue(*FilenameValue) bool    { return true }
func (BaseVisitor) VisitFilename(*Template)                   {}
func (BaseVisitor) VisitFilter(*Filter) bool                  { return true }
func (BaseVisitor) VisitFilterKind(FilterValue)               {}
func (BaseVisitor) VisitHeader(*KeyValue) bool                { return true }
func (BaseVisitor) VisitHex(*Hex) bool                        { return true }
func (BaseVisitor) VisitHexValue([]byte, string)              {}
func (BaseVisitor) VisitHurlFile(*HurlFile) bool              { return true }
func (BaseVisitor) VisitIntegerValue(IntegerValue) bool       { return true }
func (BaseVisitor) VisitI64(int64)                            {}
func (BaseVisitor) VisitJSONBody(JSONValue)                   {}
func (BaseVisitor) VisitKeyValue(*KeyValue) bool              { return true }
func (BaseVisitor) VisitLineTerminator(*LineTerminator) bool  { return true }
func (BaseVisitor) VisitLiteral(string)                       {}
func (BaseVisitor) VisitMethod(Method)                        {}
func (BaseVisitor) VisitMultilineString(*MultilineString)     {}
func (BaseVisitor) VisitNaturalOption(NaturalOption) bool     { return true }
func (BaseVisitor) VisitNot(string)                           {}
func (BaseVisitor) VisitNull(string)                          {}
func (BaseVisitor) VisitNumber(Number)                        {}
func (BaseVisitor) VisitPlaceholder(*Placeholder)             {}
func (BaseVisitor) VisitPredicate(*Predicate) bool            { return true }
func (BaseVisitor) VisitPredicateKind(PredicateFuncValue)     {}
func (BaseVisitor) VisitPredicateValue(PredicateValue) bool   { return true }
func (BaseVisitor) VisitQuery(*Query) bool                    { return true }
func (BaseVisitor) VisitQueryKind(QueryValue)                 {}
func (BaseVisitor) VisitRequest(*Request) bool                { return true }
func (BaseVisitor) VisitResponse(*Response) bool              { return true }
func (BaseVisitor) VisitRegex(*Regex)                         {}
func (BaseVisitor) VisitSection(*Section) bool                { return true }
func (BaseVisitor) VisitStatus(StatusValue)                   {}
func (BaseVisitor) VisitString(string)                        {}
func (BaseVisitor) VisitSectionHeader(string)                 {}
func (BaseVisitor) VisitSectionValue(SectionValue) bool       { return true }
func (BaseVisitor) VisitTemplate(*Template)                   {}
func (BaseVisitor) VisitURL(*Template)                        {}
func (BaseVisitor) VisitU64(*U64)                             {}
func (BaseVisitor) VisitUsize(int)                            {}
func (BaseVisitor) VisitVariableDef(*VariableDefinition) bool { return true }
func (BaseVisitor) VisitVariableName(string)                  {}
func (BaseVisitor) VisitVariableValue(VariableValue) bool     { return true }
func (BaseVisitor) VisitVerbosityOption(VerbosityOption) bool { return true }
func (BaseVisitor) VisitVersion(VersionValue)                 {}
func (BaseVisitor) VisitXMLBody(string)                       {}
func (BaseVisitor) VisitWhitespace(*Whitespace)               {}

// Walk visits file with v (Rust's visitor.visit_hurl_file(file)).
func Walk(v Visitor, file *HurlFile) {
	visit(v, file, v.VisitHurlFile, WalkHurlFile)
}

func visit[T any](v Visitor, node T, enter func(T) bool, walk func(Visitor, T)) {
	if enter(node) {
		walk(v, node)
	}
}

func visitLineTerminators(v Visitor, lts []LineTerminator) {
	for i := range lts {
		visit(v, &lts[i], v.VisitLineTerminator, WalkLineTerminator)
	}
}

func visitFilters(v Visitor, filters []SpacedFilter) {
	for i := range filters {
		v.VisitWhitespace(&filters[i].Space0)
		visit(v, &filters[i].Filter, v.VisitFilter, WalkFilter)
	}
}

func visitRegexValue(v Visitor, value RegexValue) {
	switch r := value.(type) {
	case RegexTemplate:
		v.VisitTemplate(&r.Value)
	case RegexRegex:
		v.VisitRegex(&r.Value)
	}
}

func WalkAssert(v Visitor, a *Assert) {
	visitLineTerminators(v, a.LineTerminators)
	v.VisitWhitespace(&a.Space0)
	visit(v, &a.Query, v.VisitQuery, WalkQuery)
	visitFilters(v, a.Filters)
	v.VisitWhitespace(&a.Space1)
	visit(v, &a.Predicate, v.VisitPredicate, WalkPredicate)
	visit(v, &a.LineTerminator0, v.VisitLineTerminator, WalkLineTerminator)
}

func WalkBase64(v Visitor, b *Base64) {
	v.VisitLiteral("base64,")
	v.VisitWhitespace(&b.Space0)
	v.VisitBase64Value(b.Value, b.Source)
	v.VisitWhitespace(&b.Space1)
	v.VisitLiteral(";")
}

func WalkBody(v Visitor, b *Body) {
	visitLineTerminators(v, b.LineTerminators)
	v.VisitWhitespace(&b.Space0)
	switch value := b.Value.(type) {
	case BytesJSON:
		v.VisitJSONBody(value.Value)
	case BytesXML:
		v.VisitXMLBody(value.Value)
	case BytesMultilineString:
		v.VisitMultilineString(&value.Value)
	case BytesOnelineString:
		v.VisitTemplate(&value.Value)
	case BytesBase64:
		visit(v, &value.Value, v.VisitBase64, WalkBase64)
	case BytesFile:
		visit(v, &value.Value, v.VisitFile, WalkFile)
	case BytesHex:
		visit(v, &value.Value, v.VisitHex, WalkHex)
	}
	visit(v, &b.LineTerminator0, v.VisitLineTerminator, WalkLineTerminator)
}

func WalkBoolOption(v Visitor, o BooleanOption) {
	switch o := o.(type) {
	case BooleanLiteral:
		v.VisitBool(o.Value)
	case BooleanPlaceholder:
		v.VisitPlaceholder(&o.Value)
	}
}

func WalkCapture(v Visitor, c *Capture) {
	visitLineTerminators(v, c.LineTerminators)
	v.VisitWhitespace(&c.Space0)
	v.VisitTemplate(&c.Name)
	v.VisitWhitespace(&c.Space1)
	v.VisitLiteral(":")
	v.VisitWhitespace(&c.Space2)
	visit(v, &c.Query, v.VisitQuery, WalkQuery)
	visitFilters(v, c.Filters)
	if c.Redacted {
		v.VisitWhitespace(&c.Space3)
		// A string rather than a literal, for compatibility with the <= 6.1.1 HTML export.
		v.VisitString("redact")
	}
	visit(v, &c.LineTerminator0, v.VisitLineTerminator, WalkLineTerminator)
}

func WalkCookie(v Visitor, c *Cookie) {
	visitLineTerminators(v, c.LineTerminators)
	v.VisitWhitespace(&c.Space0)
	v.VisitTemplate(&c.Name)
	v.VisitWhitespace(&c.Space1)
	v.VisitLiteral(":")
	v.VisitWhitespace(&c.Space2)
	v.VisitTemplate(&c.Value)
	visit(v, &c.LineTerminator0, v.VisitLineTerminator, WalkLineTerminator)
}

func WalkCount(v Visitor, c Count) {
	switch c := c.(type) {
	case CountFinite:
		v.VisitUsize(c.Value)
	case CountInfinite:
		v.VisitI64(-1)
	}
}

func WalkCountOption(v Visitor, o CountOption) {
	switch o := o.(type) {
	case CountLiteral:
		visit(v, o.Value, v.VisitCount, WalkCount)
	case CountPlaceholder:
		v.VisitPlaceholder(&o.Value)
	}
}

func WalkDuration(v Visitor, d *Duration) {
	v.VisitU64(&d.Value)
	if d.Unit != nil {
		v.VisitDurationUnit(*d.Unit)
	}
}

func WalkDurationOption(v Visitor, o DurationOption) {
	switch o := o.(type) {
	case DurationLiteral:
		visit(v, &o.Value, v.VisitDuration, WalkDuration)
	case DurationPlaceholder:
		v.VisitPlaceholder(&o.Value)
	}
}

func WalkEntry(v Visitor, e *Entry) {
	visit(v, &e.Request, v.VisitRequest, WalkRequest)
	if e.Response != nil {
		visit(v, e.Response, v.VisitResponse, WalkResponse)
	}
}

func WalkEntryOption(v Visitor, o *EntryOption) {
	visitLineTerminators(v, o.LineTerminators)
	v.VisitWhitespace(&o.Space0)
	v.VisitString(o.Kind.Identifier())
	v.VisitWhitespace(&o.Space1)
	v.VisitLiteral(":")
	v.VisitWhitespace(&o.Space2)
	switch k := o.Kind.(type) {
	case OptionAWSSigV4:
		v.VisitTemplate(&k.Value)
	case OptionCACertificate:
		v.VisitFilename(&k.Value)
	case OptionClientCert:
		v.VisitFilename(&k.Value)
	case OptionClientKey:
		v.VisitFilename(&k.Value)
	case OptionCompressed:
		visit(v, k.Value, v.VisitBoolOption, WalkBoolOption)
	case OptionConnectTo:
		v.VisitTemplate(&k.Value)
	case OptionConnectTimeout:
		visit(v, k.Value, v.VisitDurationOption, WalkDurationOption)
	case OptionDelay:
		visit(v, k.Value, v.VisitDurationOption, WalkDurationOption)
	case OptionDigest:
		visit(v, k.Value, v.VisitBoolOption, WalkBoolOption)
	case OptionFailWithBody:
		visit(v, k.Value, v.VisitBoolOption, WalkBoolOption)
	case OptionFollowLocation:
		visit(v, k.Value, v.VisitBoolOption, WalkBoolOption)
	case OptionFollowLocationTrusted:
		visit(v, k.Value, v.VisitBoolOption, WalkBoolOption)
	case OptionHeader:
		v.VisitTemplate(&k.Value)
	case OptionHTTP10:
		visit(v, k.Value, v.VisitBoolOption, WalkBoolOption)
	case OptionHTTP11:
		visit(v, k.Value, v.VisitBoolOption, WalkBoolOption)
	case OptionHTTP2:
		visit(v, k.Value, v.VisitBoolOption, WalkBoolOption)
	case OptionHTTP2PriorKnowledge:
		visit(v, k.Value, v.VisitBoolOption, WalkBoolOption)
	case OptionHTTP3:
		visit(v, k.Value, v.VisitBoolOption, WalkBoolOption)
	case OptionInsecure:
		visit(v, k.Value, v.VisitBoolOption, WalkBoolOption)
	case OptionIPv4:
		visit(v, k.Value, v.VisitBoolOption, WalkBoolOption)
	case OptionIPv6:
		visit(v, k.Value, v.VisitBoolOption, WalkBoolOption)
	case OptionLimitRate:
		visit(v, k.Value, v.VisitNaturalOption, WalkNaturalOption)
	case OptionMaxRedirect:
		visit(v, k.Value, v.VisitCountOption, WalkCountOption)
	case OptionMaxTime:
		visit(v, k.Value, v.VisitDurationOption, WalkDurationOption)
	case OptionNegotiate:
		visit(v, k.Value, v.VisitBoolOption, WalkBoolOption)
	case OptionNetRc:
		visit(v, k.Value, v.VisitBoolOption, WalkBoolOption)
	case OptionNetRcFile:
		v.VisitFilename(&k.Value)
	case OptionNetRcOptional:
		visit(v, k.Value, v.VisitBoolOption, WalkBoolOption)
	case OptionNoHeader:
		v.VisitTemplate(&k.Value)
	case OptionNoJSONPathCoercion:
		visit(v, k.Value, v.VisitBoolOption, WalkBoolOption)
	case OptionNtlm:
		visit(v, k.Value, v.VisitBoolOption, WalkBoolOption)
	case OptionOutput:
		v.VisitFilename(&k.Value)
	case OptionPathAsIs:
		visit(v, k.Value, v.VisitBoolOption, WalkBoolOption)
	case OptionPinnedPublicKey:
		v.VisitTemplate(&k.Value)
	case OptionProxy:
		v.VisitTemplate(&k.Value)
	case OptionRepeat:
		visit(v, k.Value, v.VisitCountOption, WalkCountOption)
	case OptionResolve:
		v.VisitTemplate(&k.Value)
	case OptionRetry:
		visit(v, k.Value, v.VisitCountOption, WalkCountOption)
	case OptionRetryInterval:
		visit(v, k.Value, v.VisitDurationOption, WalkDurationOption)
	case OptionSkip:
		visit(v, k.Value, v.VisitBoolOption, WalkBoolOption)
	case OptionTLSMax:
		v.VisitTemplate(&k.Value)
	case OptionTLSMin:
		v.VisitTemplate(&k.Value)
	case OptionUnixSocket:
		v.VisitFilename(&k.Value)
	case OptionUser:
		v.VisitTemplate(&k.Value)
	case OptionVariable:
		visit(v, &k.Value, v.VisitVariableDef, WalkVariableDef)
	case OptionVerbose:
		visit(v, k.Value, v.VisitBoolOption, WalkBoolOption)
	case OptionVerbosity:
		visit(v, k.Value, v.VisitVerbosityOption, WalkVerbosityOption)
	case OptionVeryVerbose:
		visit(v, k.Value, v.VisitBoolOption, WalkBoolOption)
	}
	visit(v, &o.LineTerminator0, v.VisitLineTerminator, WalkLineTerminator)
}

func WalkFile(v Visitor, f *File) {
	v.VisitLiteral("file,")
	v.VisitWhitespace(&f.Space0)
	v.VisitFilename(&f.Filename)
	v.VisitWhitespace(&f.Space1)
	v.VisitLiteral(";")
}

func WalkFilter(v Visitor, f *Filter) {
	v.VisitFilterKind(f.Value)
	switch k := f.Value.(type) {
	case FilterCharsetDecode:
		v.VisitWhitespace(&k.Space0)
		v.VisitTemplate(&k.Encoding)
	case FilterDecode:
		v.VisitWhitespace(&k.Space0)
		v.VisitTemplate(&k.Encoding)
	case FilterFormat:
		v.VisitWhitespace(&k.Space0)
		v.VisitTemplate(&k.Fmt)
	case FilterDateFormat:
		v.VisitWhitespace(&k.Space0)
		v.VisitTemplate(&k.Fmt)
	case FilterJSONPath:
		v.VisitWhitespace(&k.Space0)
		v.VisitTemplate(&k.Expr)
	case FilterNth:
		v.VisitWhitespace(&k.Space0)
		visit(v, k.N, v.VisitIntegerValue, WalkIntegerValue)
	case FilterRegex:
		v.VisitWhitespace(&k.Space0)
		visitRegexValue(v, k.Value)
	case FilterReplace:
		v.VisitWhitespace(&k.Space0)
		v.VisitTemplate(&k.OldValue)
		v.VisitWhitespace(&k.Space1)
		v.VisitTemplate(&k.NewValue)
	case FilterReplaceRegex:
		v.VisitWhitespace(&k.Space0)
		visitRegexValue(v, k.Pattern)
		v.VisitWhitespace(&k.Space1)
		v.VisitTemplate(&k.NewValue)
	case FilterSplit:
		v.VisitWhitespace(&k.Space0)
		v.VisitTemplate(&k.Sep)
	case FilterToDate:
		v.VisitWhitespace(&k.Space0)
		v.VisitTemplate(&k.Fmt)
	case FilterURLQueryParam:
		v.VisitWhitespace(&k.Space0)
		v.VisitTemplate(&k.Param)
	case FilterXPath:
		v.VisitWhitespace(&k.Space0)
		v.VisitTemplate(&k.Expr)
	}
}

func WalkFilenameParam(v Visitor, p *FilenameParam) {
	visitLineTerminators(v, p.LineTerminators)
	v.VisitWhitespace(&p.Space0)
	v.VisitTemplate(&p.Key)
	v.VisitWhitespace(&p.Space1)
	v.VisitLiteral(":")
	v.VisitWhitespace(&p.Space2)
	visit(v, &p.Value, v.VisitFilenameValue, WalkFilenameValue)
	visit(v, &p.LineTerminator0, v.VisitLineTerminator, WalkLineTerminator)
}

func WalkFilenameValue(v Visitor, fv *FilenameValue) {
	v.VisitLiteral("file,")
	v.VisitWhitespace(&fv.Space0)
	v.VisitFilename(&fv.Filename)
	v.VisitWhitespace(&fv.Space1)
	v.VisitLiteral(";")
	v.VisitWhitespace(&fv.Space2)
	if fv.ContentType != nil {
		v.VisitTemplate(fv.ContentType)
	}
}

func WalkHeader(v Visitor, h *KeyValue) {
	visit(v, h, v.VisitKeyValue, WalkKeyValue)
}

func WalkHex(v Visitor, h *Hex) {
	v.VisitLiteral("hex,")
	v.VisitWhitespace(&h.Space0)
	v.VisitHexValue(h.Value, h.Source)
	v.VisitWhitespace(&h.Space1)
	v.VisitLiteral(";")
}

func WalkHurlFile(v Visitor, f *HurlFile) {
	for i := range f.Entries {
		visit(v, &f.Entries[i], v.VisitEntry, WalkEntry)
	}
	visitLineTerminators(v, f.LineTerminators)
}

func WalkIntegerValue(v Visitor, n IntegerValue) {
	switch n := n.(type) {
	case IntegerLiteral:
		v.VisitI64(n.Value.Value)
	case IntegerPlaceholder:
		v.VisitPlaceholder(&n.Value)
	}
}

func WalkKeyValue(v Visitor, kv *KeyValue) {
	visitLineTerminators(v, kv.LineTerminators)
	v.VisitWhitespace(&kv.Space0)
	v.VisitTemplate(&kv.Key)
	v.VisitWhitespace(&kv.Space1)
	v.VisitLiteral(":")
	v.VisitWhitespace(&kv.Space2)
	v.VisitTemplate(&kv.Value)
	visit(v, &kv.LineTerminator0, v.VisitLineTerminator, WalkLineTerminator)
}

func WalkLineTerminator(v Visitor, lt *LineTerminator) {
	v.VisitWhitespace(&lt.Space0)
	if lt.Comment != nil {
		v.VisitComment(lt.Comment)
	}
	v.VisitWhitespace(&lt.Newline)
}

func WalkNaturalOption(v Visitor, o NaturalOption) {
	switch o := o.(type) {
	case NaturalLiteral:
		v.VisitU64(&o.Value)
	case NaturalPlaceholder:
		v.VisitPlaceholder(&o.Value)
	}
}

func WalkQuery(v Visitor, q *Query) {
	v.VisitQueryKind(q.Value)
	switch k := q.Value.(type) {
	case QueryHeader:
		v.VisitWhitespace(&k.Space0)
		v.VisitTemplate(&k.Name)
	case QueryCookie:
		v.VisitWhitespace(&k.Space0)
		v.VisitCookiePath(&k.Expr)
	case QueryXPath:
		v.VisitWhitespace(&k.Space0)
		v.VisitTemplate(&k.Expr)
	case QueryJSONPath:
		v.VisitWhitespace(&k.Space0)
		v.VisitTemplate(&k.Expr)
	case QueryRegex:
		v.VisitWhitespace(&k.Space0)
		visitRegexValue(v, k.Value)
	case QueryVariable:
		v.VisitWhitespace(&k.Space0)
		v.VisitTemplate(&k.Name)
	case QueryCertificate:
		v.VisitWhitespace(&k.Space0)
		v.VisitString(k.AttributeName.ToSource())
	}
}

func WalkPredicate(v Visitor, p *Predicate) {
	if p.Not {
		v.VisitNot("not")
		v.VisitWhitespace(&p.Space0)
	}
	kind := p.PredicateFunc.Value
	v.VisitPredicateKind(kind)
	var space0 *Whitespace
	var value PredicateValue
	switch k := kind.(type) {
	case PredicateFuncEqual:
		space0, value = &k.Space0, k.Value
	case PredicateFuncNotEqual:
		space0, value = &k.Space0, k.Value
	case PredicateFuncGreaterThan:
		space0, value = &k.Space0, k.Value
	case PredicateFuncGreaterThanOrEqual:
		space0, value = &k.Space0, k.Value
	case PredicateFuncLessThan:
		space0, value = &k.Space0, k.Value
	case PredicateFuncLessThanOrEqual:
		space0, value = &k.Space0, k.Value
	case PredicateFuncStartWith:
		space0, value = &k.Space0, k.Value
	case PredicateFuncEndWith:
		space0, value = &k.Space0, k.Value
	case PredicateFuncContain:
		space0, value = &k.Space0, k.Value
	case PredicateFuncInclude:
		space0, value = &k.Space0, k.Value
	case PredicateFuncMatch:
		space0, value = &k.Space0, k.Value
	default:
		return
	}
	v.VisitWhitespace(space0)
	visit(v, value, v.VisitPredicateValue, WalkPredicateValue)
}

func WalkPredicateValue(v Visitor, pv PredicateValue) {
	switch pv := pv.(type) {
	case PredicateBase64:
		visit(v, &pv.Value, v.VisitBase64, WalkBase64)
	case PredicateBool:
		v.VisitBool(pv.Value)
	case PredicateFile:
		visit(v, &pv.Value, v.VisitFile, WalkFile)
	case PredicateHex:
		visit(v, &pv.Value, v.VisitHex, WalkHex)
	case PredicateMultilineString:
		v.VisitMultilineString(&pv.Value)
	case PredicateNull:
		v.VisitNull("null")
	case PredicateNumber:
		v.VisitNumber(pv.Value)
	case PredicatePlaceholder:
		v.VisitPlaceholder(&pv.Value)
	case PredicateRegex:
		v.VisitRegex(&pv.Value)
	case PredicateString:
		v.VisitTemplate(&pv.Value)
	}
}

func WalkRequest(v Visitor, r *Request) {
	visitLineTerminators(v, r.LineTerminators)
	v.VisitWhitespace(&r.Space0)
	v.VisitMethod(r.Method)
	v.VisitWhitespace(&r.Space1)
	v.VisitURL(&r.URL)
	visit(v, &r.LineTerminator0, v.VisitLineTerminator, WalkLineTerminator)
	for i := range r.Headers {
		visit(v, &r.Headers[i], v.VisitKeyValue, WalkKeyValue)
	}
	for i := range r.Sections {
		visit(v, &r.Sections[i], v.VisitSection, WalkSection)
	}
	if r.Body != nil {
		visit(v, r.Body, v.VisitBody, WalkBody)
	}
}

func WalkResponse(v Visitor, r *Response) {
	visitLineTerminators(v, r.LineTerminators)
	v.VisitWhitespace(&r.Space0)
	v.VisitVersion(r.Version.Value)
	v.VisitWhitespace(&r.Space1)
	v.VisitStatus(r.Status.Value)
	visit(v, &r.LineTerminator0, v.VisitLineTerminator, WalkLineTerminator)
	for i := range r.Headers {
		visit(v, &r.Headers[i], v.VisitKeyValue, WalkKeyValue)
	}
	for i := range r.Sections {
		visit(v, &r.Sections[i], v.VisitSection, WalkSection)
	}
	if r.Body != nil {
		visit(v, r.Body, v.VisitBody, WalkBody)
	}
}

func WalkSection(v Visitor, s *Section) {
	visitLineTerminators(v, s.LineTerminators)
	v.VisitWhitespace(&s.Space0)
	v.VisitSectionHeader("[" + s.Identifier() + "]")
	visit(v, &s.LineTerminator0, v.VisitLineTerminator, WalkLineTerminator)
	visit(v, s.Value, v.VisitSectionValue, WalkSectionValue)
}

func WalkSectionValue(v Visitor, sv SectionValue) {
	switch sv := sv.(type) {
	case SectionAsserts:
		for i := range sv.Value {
			visit(v, &sv.Value[i], v.VisitAssert, WalkAssert)
		}
	case SectionBasicAuth:
		if sv.Value != nil {
			visit(v, sv.Value, v.VisitKeyValue, WalkKeyValue)
		}
	case SectionCaptures:
		for i := range sv.Value {
			visit(v, &sv.Value[i], v.VisitCapture, WalkCapture)
		}
	case SectionCookies:
		for i := range sv.Value {
			visit(v, &sv.Value[i], v.VisitCookie, WalkCookie)
		}
	case SectionFormParams:
		for i := range sv.Value {
			visit(v, &sv.Value[i], v.VisitKeyValue, WalkKeyValue)
		}
	case SectionMultipartFormData:
		for _, p := range sv.Value {
			switch p := p.(type) {
			case MultipartParamParam:
				visit(v, &p.Value, v.VisitKeyValue, WalkKeyValue)
			case MultipartParamFilenameParam:
				visit(v, &p.Value, v.VisitFilenameParam, WalkFilenameParam)
			}
		}
	case SectionOptions:
		for i := range sv.Value {
			visit(v, &sv.Value[i], v.VisitEntryOption, WalkEntryOption)
		}
	case SectionQueryParams:
		for i := range sv.Value {
			visit(v, &sv.Value[i], v.VisitKeyValue, WalkKeyValue)
		}
	}
}

func WalkVariableDef(v Visitor, d *VariableDefinition) {
	v.VisitVariableName(d.Name)
	v.VisitWhitespace(&d.Space0)
	v.VisitLiteral("=")
	v.VisitWhitespace(&d.Space1)
	visit(v, d.Value, v.VisitVariableValue, WalkVariableValue)
}

func WalkVariableValue(v Visitor, vv VariableValue) {
	switch vv := vv.(type) {
	case VariableNull:
		v.VisitNull("null")
	case VariableBool:
		v.VisitBool(vv.Value)
	case VariableNumber:
		v.VisitNumber(vv.Value)
	case VariableString:
		v.VisitTemplate(&vv.Value)
	}
}

func WalkVerbosityOption(v Visitor, o VerbosityOption) {
	v.VisitString(o.Identifier())
}
