// Package ast is the lossless Hurl syntax tree, ported from hurl_core's ast
// and types modules. Every Whitespace, LineTerminator and Comment is kept so
// that a file can be formatted back to its exact source.
//
// Conventions (follow these when building or consuming the tree):
//
//   - Payload enums are sealed interfaces (unexported marker method) with one
//     struct per variant, stored as values (not pointers) and consumed with
//     type switches. C-like enums are int types with iota constants.
//   - Variant names are <Prefix><Variant>, where Prefix is the Rust enum name
//     with a trailing Value, Kind, Option, Element or Name removed:
//     QueryValue::Header → QueryHeader, OptionKind::Retry → OptionRetry,
//     BooleanOption::Literal → BooleanLiteral, TemplateElement::String →
//     TemplateString, PredicateFuncValue::Equal → PredicateFuncEqual,
//     PredicateValue::Null → PredicateNull, Count::Infinite → CountInfinite,
//     CertificateAttributeName::Subject → CertificateAttributeSubject.
//     Constants use the same scheme: VersionValue::Version11 → Version11,
//     DurationUnit::Second → DurationUnitSecond.
//   - Common initialisms are upper-cased everywhere: URL, HTTP, JSON, XML,
//     HTML, IP, UUID, UTF8, MD5, SHA256, AWS, CA, TLS, ISO, GraphQL, XPath,
//     JSONPath (QueryValue::Jsonpath → QueryJSONPath, OptionKind::IpV4 →
//     OptionIPv4, JsonValue → JSONValue, GraphQl → GraphQL).
//   - Named variant fields keep their Rust names in CamelCase (Space0, Expr,
//     OldValue, AttributeName). A single tuple payload is the field Value;
//     the "short syntax" bool of QueryParams/FormParams/MultipartFormData is
//     Short (SectionQueryParams{Value: params, Short: true}).
//   - Option<T> is *T for structs and C-like enums, nil for an absent
//     interface (e.g. Request.Body *Body, FilenameValue.ContentType *Template,
//     Duration.Unit *DurationUnit). Template.Delimiter is a rune, 0 for none.
//   - SourceString is plain string; ToSource is a ToSource() string method;
//     Display is String(). Rust's Vec<(Whitespace, Filter)> is []SpacedFilter.
//   - Exception: CookieAttributeName is a struct {Kind, Value} rather than a
//     sealed interface, since every variant carries the same string payload.
//
// The Visitor (visit.go) mirrors Rust's visit.rs; see its doc for how
// composite visits and Walk* functions interact.
package ast
