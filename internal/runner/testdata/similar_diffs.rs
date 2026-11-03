// Generates similar_diffs.jsonl.gz, the reference diffs of TestDiffSimilar, with
// the similar crate used by Hurl 8.1.0. In a cargo project depending on
// similar = "=3.2.1", with this file as src/main.rs:
//
//     cargo run --release | gzip -9 > similar_diffs.jsonl.gz

use similar::{ChangeTag, DiffOp, DiffTag, TextDiff};

fn get_hunk_lines(ops: &[DiffOp]) -> (isize, isize) {
    let mut start = 0isize;
    for op in ops {
        match op.tag() {
            DiffTag::Equal => start = op.old_range().start as isize,
            DiffTag::Delete => return (start, op.old_range().start as isize),
            DiffTag::Insert => return (start, op.old_range().start as isize - 1),
            DiffTag::Replace => return (start, op.old_range().start as isize),
        }
    }
    (start, 0)
}

fn esc(s: &str) -> String {
    let mut o = String::from("\"");
    for c in s.chars() {
        match c {
            '\n' => o.push_str("\\n"),
            '\r' => o.push_str("\\r"),
            '"' => o.push_str("\\\""),
            '\\' => o.push_str("\\\\"),
            c => o.push(c),
        }
    }
    o.push('"');
    o
}

struct Rng(u64);
impl Rng {
    fn next(&mut self) -> u64 {
        self.0 = self.0.wrapping_mul(6364136223846793005).wrapping_add(1442695040888963407);
        self.0 >> 33
    }
    fn below(&mut self, n: u64) -> u64 { self.next() % n }
}

fn main() {
    let vocab = ["a\n", "b\n", "c\n", "d\n", "{\n", "}\n", "x\r\n", "\n"];
    let mut rng = Rng(42);
    for case in 0..3000 {
        let k = 2 + rng.below(vocab.len() as u64 - 1) as usize;
        let len = if case % 50 == 0 { 100 + rng.below(150) } else { rng.below(30) } as usize;
        let old: Vec<&str> = (0..len).map(|_| vocab[rng.below(k as u64) as usize]).collect();
        let mut new = old.clone();
        let edits = 1 + rng.below(6);
        for _ in 0..edits {
            let op = rng.below(3);
            let pos = if new.is_empty() { 0 } else { rng.below(new.len() as u64 + 1) as usize };
            match op {
                0 => new.insert(pos, vocab[rng.below(k as u64) as usize]),
                1 if pos < new.len() => { new.remove(pos); }
                _ if pos < new.len() => new[pos] = vocab[rng.below(k as u64) as usize],
                _ => {}
            }
        }
        let mut old_s: String = old.concat();
        let mut new_s: String = new.concat();
        if case % 7 == 0 { old_s.push_str("tail"); }
        if case % 11 == 0 { new_s.push_str("tail2"); }
        let text_diff = TextDiff::from_lines(&old_s, &new_s);
        let mut unified = text_diff.unified_diff();
        let unified = unified.context_radius(0);
        let mut hunks = vec![];
        for hunk in unified.iter_hunks() {
            let (start, source_line) = get_hunk_lines(hunk.ops());
            let mut content = String::new();
            for change in hunk.iter_changes() {
                let sign = match change.tag() {
                    ChangeTag::Delete => "-",
                    ChangeTag::Insert => "+",
                    ChangeTag::Equal => " ",
                };
                content.push_str(&format!("{sign}{change}"));
            }
            hunks.push(format!("{{\"content\":{},\"start\":{},\"source_line\":{}}}", esc(&content), start, source_line));
        }
        println!("{{\"old\":{},\"new\":{},\"hunks\":[{}]}}", esc(&old_s), esc(&new_s), hunks.join(","));
    }
}
