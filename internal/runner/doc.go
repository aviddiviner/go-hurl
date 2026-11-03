// Package runner runs Hurl files, ported from hurl's runner module. It holds
// the values, templates, filters, queries, predicates and asserts.
//
// The core layer (values, numbers, errors, variables, templates,
// expressions, functions, JSON/multiline/body evaluation, regex, XPath glue
// and every filter as filter_*.go) follows these conventions, on top of the
// ones in ast/doc.go:
//
//   - Value is a sealed interface: ValueBool, ValueBytes, ValueDate (a
//     time.Time, always UTC), ValueHTTPResponse, ValueList, ValueNodeset,
//     ValueNull, ValueNumber, ValueObject ([]KeyValue, ordered like Rust's
//     Vec<(String, Value)>), ValueRegex, ValueString and ValueUnit. A tuple
//     payload is the field Value (ValueString{Value: "a"}); NewString,
//     NewInteger, NewFloat and NewBool are shortcuts.
//   - Number is a sealed interface: NumberFloat, NumberInteger and
//     NumberBigInteger (a decimal string). cmpNumber is Number::cmp_value.
//   - Rust's PartialEq on Value is Equal (Value::Regex and
//     Value::HttpResponse are never equal, as in Rust). Display is String().
//     Value methods from value.rs/value_impl.rs that the predicates use are
//     unexported functions prefixed with value: valueCompare, valueContains,
//     valueIsMatch, valueCount...
//   - ValueKind constants are ValueKind<Variant> (ValueKindFloat,
//     ValueKindHTTPResponse...). EvalError is EvalErrorType or
//     EvalErrorInvalidRegex, returned as an error.
//   - RunnerErrorKind::X is RunnerErrorX (RunnerErrorAssertFailure,
//     RunnerErrorHTTP, RunnerErrorInvalidJSON...); a tuple payload is the
//     field Value. Functions return *RunnerError. Its source info is the
//     unexported field sourceInfo, read with SourceInfo() (the method
//     required by sourceerr.DisplaySourceError clashes with a field name).
//   - Evaluation functions keep their Rust names: evalTemplate, evalExpr
//     (expr::eval), renderExpr (expr::render), evalFunction, evalJSONValue,
//     evalMultiline, evalBody, evalBytes, evalFile, evalRegexValue,
//     evalFilters, evalFilter, evalJSONPathJSON, evalXPathDoc and one
//     eval<Filter> per filter. Filters return (Value, *RunnerError) where a
//     nil Value is Rust's None.
//   - filter/*.rs are grouped in filter.go (mod.rs, eval.rs),
//     filter_base64.go, filter_charset_decode.go, filter_collection.go
//     (count, first, last, nth), filter_convert.go (toFloat, toInt, toHex,
//     toString, location), filter_date.go (daysAfterNow, daysBeforeNow,
//     format/dateFormat, toDate), filter_query.go (jsonpath, xpath) and
//     filter_string.go (the string and URL filters, utf8Decode/Encode).
//   - JSON documents are encoding/json values decoded with UseNumber (nil,
//     bool, json.Number, string, []any, map[string]any). valueFromJSON is
//     Value::from_json (objects in sorted key order, like serde_json without
//     preserve_order); parseJSON is serde_json::from_str.
//
// The execution layer runs files and entries:
//
//   - Run (runner::run) parses and runs a file with immediate output;
//     RunEntries (run_entries) runs parsed entries with a given stdout,
//     logger and optional EventListener (used by the parallel runner).
//   - RunnerOptions is built with DefaultRunnerOptions plus field
//     assignments (no builder). Option<String> fields are empty strings,
//     Option<Count> fields are nil ast.Count, FromEntry/ToEntry are 0 when
//     unset. Per-entry [Options] are applied by entryOptions (options.rs) and
//     mapped onto httpclient.ClientOptions by clientOptions; unsupported
//     features (HTTP/1.0, HTTP/3, NTLM, digest, AWS SigV4...) fail in the
//     HTTP client with a "not supported by go-hurl" error.
//   - HurlResult, EntryResult, CaptureResult and AssertResult (a sealed
//     interface, AssertResult<Variant>) are result.rs/assert.rs. Rust's
//     Result<T, RunnerError> fields are a value plus an *Err field.
//   - hurl_file.go (hurl_file.rs), entry.go, request.go, response.go,
//     capture.go, assert.go, predicate.go, predicate_value.go, query.go,
//     multipart.go, cache.go (bodyCache), output.go (Output is
//     util.Output), event.go.
//   - diff.go and diff_compact.go reproduce the Rust similar 3.2.1 line diff
//     used by Hurl: Myers' divide and conquer, the Compact hook
//     (cleanup_diff_ops), the Replace hook and unified hunks without context.
//     similar's heuristics for large inputs (front anchors when both changed
//     ranges exceed 96 lines and one is twice the other, preflight
//     reductions over 512 lines, approximate middle snakes past 256 edits)
//     are not ported, so diffs of large, very different bodies may report a
//     different first hunk.
package runner
