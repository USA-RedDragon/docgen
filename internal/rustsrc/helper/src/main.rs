//! Reads the module tree of Rust crates with syn and prints their items as JSON
//! for docgen.
//!
//! Usage: `docgen-rs-helper <root file>...`, where each root file is the
//! `lib.rs` or `main.rs` of a target. The output is one JSON array with one
//! object for each root file, in order.
//!
//! `docgen-rs-helper --symbols <file>...` reads each file alone, with no
//! module tree, and prints one JSON array with the places of the items of each
//! file: see [`read_symbols`].

use std::fs;
use std::path::{Path, PathBuf};
use std::process::ExitCode;

use proc_macro2::Span;
use quote::ToTokens;
use serde_json::{json, Map, Value};
use syn::spanned::Spanned;

fn main() -> ExitCode {
    let mut roots: Vec<String> = std::env::args().skip(1).collect();
    let symbols = roots.first().is_some_and(|a| a == "--symbols");
    if symbols {
        roots.remove(0);
    }
    if roots.is_empty() {
        eprintln!("usage: docgen-rs-helper [--symbols] <file>...");
        return ExitCode::from(2);
    }
    let crates: Vec<Value> = roots
        .iter()
        .map(|r| {
            if symbols {
                read_symbols(Path::new(r))
            } else {
                read_crate(Path::new(r))
            }
        })
        .collect();
    println!("{}", Value::Array(crates));
    ExitCode::SUCCESS
}

/// The place of a module: its file, the directory of its child modules, the
/// directory of a `#[path]` on a child declaration, and the cfg of it and its
/// parents.
#[derive(Clone)]
struct Place {
    path: Vec<String>,
    file: PathBuf,
    dir: PathBuf,
    path_dir: PathBuf,
    cfg: Vec<String>,
}

/// What the module tree holds about a module, other than its items.
struct Head {
    vis: String,
    doc: String,
    span: Option<Value>,
    insert: usize,
}

struct Reader {
    modules: Vec<Value>,
    errors: Vec<String>,
    warnings: Vec<String>,
    seen: Vec<PathBuf>,
}

fn read_crate(root: &Path) -> Value {
    let mut r = Reader {
        modules: Vec::new(),
        errors: Vec::new(),
        warnings: Vec::new(),
        seen: Vec::new(),
    };
    let dir = root.parent().map(Path::to_path_buf).unwrap_or_default();
    let place = Place {
        path: Vec::new(),
        file: root.to_path_buf(),
        dir: dir.clone(),
        path_dir: dir,
        cfg: Vec::new(),
    };
    match parse(root) {
        Ok((text, file)) => {
            let doc = doc_of(&file.attrs, Style::Inner, root, &mut r.warnings);
            let head = Head {
                vis: "pub".into(),
                doc,
                span: None,
                insert: text.len(),
            };
            r.module(&place, head, &file.items, &text);
        }
        Err(e) => r.errors.push(e),
    }
    json!({
        "root": root.display().to_string(),
        "modules": r.modules,
        "errors": r.errors,
        "warnings": r.warnings,
    })
}

fn parse(path: &Path) -> Result<(String, syn::File), String> {
    let text = fs::read_to_string(path).map_err(|e| format!("{}: {e}", path.display()))?;
    let file = syn::parse_file(&text).map_err(|e| {
        let at = e.span().start();
        format!("{}:{}:{}: {e}", path.display(), at.line, at.column + 1)
    })?;
    Ok((text, file))
}

impl Reader {
    fn module(&mut self, place: &Place, head: Head, items: &[syn::Item], text: &str) {
        let mut out = Vec::new();
        let mut uses = Vec::new();
        let mut defaults = Vec::new();
        for item in items {
            if let syn::Item::Impl(i) = item {
                defaults.extend(default_impl(i));
            }
            self.item(place, item, text, &mut out, &mut uses);
        }
        self.modules.push(json!({
            "path": place.path.join("::"),
            "file": place.file.display().to_string(),
            "vis": head.vis,
            "doc": head.doc,
            "cfg": place.cfg,
            "span": head.span,
            "insert": head.insert,
            "items": out,
            "uses": uses,
            "default_impls": defaults,
        }));
    }

    fn item(
        &mut self,
        place: &Place,
        item: &syn::Item,
        text: &str,
        out: &mut Vec<Value>,
        uses: &mut Vec<Value>,
    ) {
        let file = &place.file;
        match item {
            syn::Item::Const(c) => {
                let mut v = self.base(
                    "const",
                    &c.ident.to_string(),
                    &c.vis,
                    &c.attrs,
                    item.span(),
                    file,
                );
                v.insert("type".into(), src(&c.ty).into());
                v.insert("expr".into(), src(&c.expr).into());
                out.push(Value::Object(v));
            }
            syn::Item::Static(s) => {
                let mut v = self.base(
                    "static",
                    &s.ident.to_string(),
                    &s.vis,
                    &s.attrs,
                    item.span(),
                    file,
                );
                v.insert("type".into(), src(&s.ty).into());
                v.insert("expr".into(), src(&s.expr).into());
                v.insert(
                    "mut".into(),
                    matches!(s.mutability, syn::StaticMutability::Mut(_)).into(),
                );
                out.push(Value::Object(v));
            }
            syn::Item::Struct(s) => {
                let mut v = self.base(
                    "struct",
                    &s.ident.to_string(),
                    &s.vis,
                    &s.attrs,
                    item.span(),
                    file,
                );
                v.insert("generics".into(), (!s.generics.params.is_empty()).into());
                v.insert("fields".into(), self.fields(&s.fields, file));
                v.insert("style".into(), style(&s.fields).into());
                type_attrs(&mut v, &s.attrs);
                out.push(Value::Object(v));
            }
            syn::Item::Enum(e) => {
                let mut v = self.base(
                    "enum",
                    &e.ident.to_string(),
                    &e.vis,
                    &e.attrs,
                    item.span(),
                    file,
                );
                v.insert("generics".into(), (!e.generics.params.is_empty()).into());
                let variants: Vec<Value> = e
                    .variants
                    .iter()
                    .map(|var| {
                        let mut m = Map::new();
                        m.insert("name".into(), var.ident.to_string().into());
                        m.insert(
                            "doc".into(),
                            doc_of(&var.attrs, Style::Outer, file, &mut self.warnings).into(),
                        );
                        m.insert("span".into(), span_json(var.span(), file));
                        m.insert("style".into(), style(&var.fields).into());
                        m.insert("fields".into(), self.fields(&var.fields, file));
                        m.insert("cfg".into(), cfgs(&var.attrs).into());
                        if let Some(d) = &var.discriminant {
                            m.insert("discriminant".into(), src(&d.1).into());
                        }
                        if let Some(err) = error_attr(&var.attrs) {
                            m.insert("error".into(), err);
                        }
                        Value::Object(m)
                    })
                    .collect();
                v.insert("variants".into(), variants.into());
                type_attrs(&mut v, &e.attrs);
                out.push(Value::Object(v));
            }
            syn::Item::Union(u) => {
                let v = self.base(
                    "union",
                    &u.ident.to_string(),
                    &u.vis,
                    &u.attrs,
                    item.span(),
                    file,
                );
                out.push(Value::Object(v));
            }
            syn::Item::Fn(f) => {
                let mut v = self.base(
                    "fn",
                    &f.sig.ident.to_string(),
                    &f.vis,
                    &f.attrs,
                    item.span(),
                    file,
                );
                v.insert("ret".into(), ret_text(&f.sig).into());
                out.push(Value::Object(v));
            }
            syn::Item::Trait(t) => {
                let v = self.base(
                    "trait",
                    &t.ident.to_string(),
                    &t.vis,
                    &t.attrs,
                    item.span(),
                    file,
                );
                out.push(Value::Object(v));
            }
            syn::Item::Type(t) => {
                let mut v = self.base(
                    "type",
                    &t.ident.to_string(),
                    &t.vis,
                    &t.attrs,
                    item.span(),
                    file,
                );
                v.insert("type".into(), src(&t.ty).into());
                out.push(Value::Object(v));
            }
            syn::Item::Macro(m) => {
                if let Some(ident) = &m.ident {
                    let exported = m.attrs.iter().any(|a| a.path().is_ident("macro_export"));
                    let vis = if exported { "pub" } else { "" };
                    let mut v = self.base(
                        "macro",
                        &ident.to_string(),
                        &syn::Visibility::Inherited,
                        &m.attrs,
                        item.span(),
                        file,
                    );
                    v.insert("vis".into(), vis.into());
                    out.push(Value::Object(v));
                }
            }
            syn::Item::Impl(i) => self.impl_items(i, file, out),
            syn::Item::Use(u) => use_tree(&u.tree, &vis_text(&u.vis), &mut Vec::new(), uses),
            syn::Item::Mod(m) => self.submodule(place, m, text, out),
            _ => {}
        }
    }

    /// The associated consts and fns of an inherent impl of a type with a
    /// one-segment path, named `Type::name`.
    fn impl_items(&mut self, i: &syn::ItemImpl, file: &Path, out: &mut Vec<Value>) {
        if i.trait_.is_some() {
            return;
        }
        let syn::Type::Path(tp) = &*i.self_ty else {
            return;
        };
        if tp.qself.is_some() || tp.path.segments.len() != 1 {
            return;
        }
        let ty = tp.path.segments[0].ident.to_string();
        let impl_cfg = cfgs(&i.attrs);
        for ii in &i.items {
            let mut v = match ii {
                syn::ImplItem::Const(c) => {
                    let mut v = self.base(
                        "const",
                        &format!("{ty}::{}", c.ident),
                        &c.vis,
                        &c.attrs,
                        ii.span(),
                        file,
                    );
                    v.insert("type".into(), src(&c.ty).into());
                    v.insert("expr".into(), src(&c.expr).into());
                    v
                }
                syn::ImplItem::Fn(f) => {
                    let mut v = self.base(
                        "fn",
                        &format!("{ty}::{}", f.sig.ident),
                        &f.vis,
                        &f.attrs,
                        ii.span(),
                        file,
                    );
                    v.insert("ret".into(), ret_text(&f.sig).into());
                    v.insert("method".into(), method_json(f));
                    v
                }
                _ => continue,
            };
            if !impl_cfg.is_empty() {
                let mut all = impl_cfg.clone();
                if let Some(Value::Array(own)) = v.get("cfg") {
                    all.extend(own.iter().filter_map(|c| c.as_str().map(String::from)));
                }
                v.insert("cfg".into(), all.into());
            }
            v.insert("generics".into(), (!i.generics.params.is_empty()).into());
            out.push(Value::Object(v));
        }
    }

    fn submodule(&mut self, place: &Place, m: &syn::ItemMod, text: &str, out: &mut Vec<Value>) {
        let name = m.ident.unraw_string();
        let mut child = place.clone();
        child.path.push(name.clone());
        let own_cfg = cfgs(&m.attrs);
        child.cfg.extend(own_cfg.iter().cloned());
        let vis = vis_text(&m.vis);
        let mut v = self.base("mod", &name, &m.vis, &m.attrs, m.span(), &place.file);
        let path_attr = path_attr(&m.attrs);
        if let Some((brace, items)) = &m.content {
            child.dir = match &path_attr {
                Some(p) => place.dir.join(p),
                None => place.dir.join(&name),
            };
            child.path_dir = child.dir.clone();
            let doc = doc_of(&m.attrs, Style::Both, &place.file, &mut self.warnings);
            let insert = brace.span.close().byte_range().start;
            let head = Head {
                vis: vis.clone(),
                doc,
                span: Some(span_json(m.span(), &place.file)),
                insert,
            };
            self.module(&child, head, items, text);
            v.insert("inline".into(), true.into());
            out.push(Value::Object(v));
            return;
        }
        let file = match &path_attr {
            Some(p) => Some(place.path_dir.join(p)),
            None => {
                let flat = place.dir.join(format!("{name}.rs"));
                let nested = place.dir.join(&name).join("mod.rs");
                match (flat.is_file(), nested.is_file()) {
                    (true, true) => {
                        self.errors.push(format!(
                            "module {} has two files, {} and {}",
                            child.path.join("::"),
                            flat.display(),
                            nested.display()
                        ));
                        None
                    }
                    (true, false) => Some(flat),
                    (false, true) => Some(nested),
                    (false, false) => {
                        let msg = format!(
                            "module {} has no file {} or {}",
                            child.path.join("::"),
                            flat.display(),
                            nested.display()
                        );
                        if child.cfg.is_empty() {
                            self.errors.push(msg);
                        } else {
                            self.warnings.push(msg);
                        }
                        None
                    }
                }
            }
        };
        v.insert("inline".into(), false.into());
        let Some(file) = file else {
            out.push(Value::Object(v));
            return;
        };
        v.insert("file".into(), file.display().to_string().into());
        out.push(Value::Object(v));
        if self.seen.contains(&file) {
            return;
        }
        self.seen.push(file.clone());
        let (text, parsed) = match parse(&file) {
            Ok(x) => x,
            Err(e) => {
                if child.cfg.is_empty() {
                    self.errors.push(e);
                } else {
                    self.warnings.push(e);
                }
                return;
            }
        };
        let mod_rs = path_attr.is_some() || file.file_name().is_some_and(|f| f == "mod.rs");
        let parent = file.parent().map(Path::to_path_buf).unwrap_or_default();
        child.file = file.clone();
        child.path_dir = parent.clone();
        child.dir = if mod_rs { parent } else { parent.join(&name) };
        let mut doc = doc_of(&m.attrs, Style::Outer, &place.file, &mut self.warnings);
        let inner = doc_of(&parsed.attrs, Style::Inner, &file, &mut self.warnings);
        if !inner.is_empty() {
            if !doc.is_empty() {
                doc.push('\n');
            }
            doc.push_str(&inner);
        }
        let head = Head {
            vis,
            doc,
            span: Some(span_json(m.span(), &place.file)),
            insert: text.len(),
        };
        self.module(&child, head, &parsed.items, &text);
    }

    fn base(
        &mut self,
        kind: &str,
        name: &str,
        vis: &syn::Visibility,
        attrs: &[syn::Attribute],
        span: Span,
        file: &Path,
    ) -> Map<String, Value> {
        let mut v = Map::new();
        v.insert("kind".into(), kind.into());
        v.insert("name".into(), name.trim_start_matches("r#").into());
        v.insert("vis".into(), vis_text(vis).into());
        v.insert(
            "doc".into(),
            doc_of(attrs, Style::Outer, file, &mut self.warnings).into(),
        );
        v.insert("cfg".into(), cfgs(attrs).into());
        v.insert("span".into(), span_json(span, file));
        v
    }

    fn fields(&mut self, fields: &syn::Fields, file: &Path) -> Value {
        let list: Vec<Value> = fields
            .iter()
            .enumerate()
            .map(|(i, f)| {
                let name = f
                    .ident
                    .as_ref()
                    .map_or_else(|| i.to_string(), |id| id.unraw_string());
                let mut m = Map::new();
                m.insert("name".into(), name.into());
                m.insert("type".into(), src(&f.ty).into());
                m.insert("vis".into(), vis_text(&f.vis).into());
                m.insert(
                    "doc".into(),
                    doc_of(&f.attrs, Style::Outer, file, &mut self.warnings).into(),
                );
                m.insert("from".into(), has_attr(&f.attrs, "from").into());
                m.insert("source".into(), has_attr(&f.attrs, "source").into());
                m.insert("backtrace".into(), has_attr(&f.attrs, "backtrace").into());
                m.insert("cfg".into(), cfgs(&f.attrs).into());
                let conf = configulator_attrs(&f.attrs);
                if !conf.is_empty() {
                    m.insert("configulator".into(), conf.into());
                }
                if let Some(v) = serde_json_of(&f.attrs) {
                    m.insert("serde".into(), v);
                }
                Value::Object(m)
            })
            .collect();
        list.into()
    }
}

/// The signature and the body of a method, for docgen:states: its `receiver`
/// (`""` for none, `ref`, `mut`, `value`, or `typed` for `self: Type`), its
/// `params` with an identifier pattern, `async`, `const`, `generics`, and the byte offsets of its
/// body: `body_start` after the opening brace and any inner attributes, and
/// `body_end` at the closing brace.
fn method_json(f: &syn::ImplItemFn) -> Value {
    let receiver = match f.sig.receiver() {
        None => "",
        Some(r) if r.colon_token.is_some() => "typed",
        Some(r) => match (&r.reference, &r.mutability) {
            (Some(_), Some(_)) => "mut",
            (Some(_), None) => "ref",
            (None, _) => "value",
        },
    };
    let params: Vec<Value> = f
        .sig
        .inputs
        .iter()
        .filter_map(|a| match a {
            syn::FnArg::Typed(pt) => match &*pt.pat {
                syn::Pat::Ident(pi) if pi.by_ref.is_none() && pi.subpat.is_none() => {
                    Some(json!({ "name": pi.ident.to_string(), "type": src(&pt.ty) }))
                }
                _ => None,
            },
            syn::FnArg::Receiver(_) => None,
        })
        .collect();
    let mut body_start = f.block.brace_token.span.open().byte_range().end;
    for a in &f.attrs {
        if matches!(a.style, syn::AttrStyle::Inner(_)) {
            body_start = body_start.max(a.span().byte_range().end);
        }
    }
    json!({
        "receiver": receiver,
        "params": params,
        "async": f.sig.asyncness.is_some(),
        "const": f.sig.constness.is_some(),
        "generics": !f.sig.generics.params.is_empty(),
        "body_start": body_start,
        "body_end": f.block.brace_token.span.close().byte_range().start,
    })
}

trait UnrawString {
    fn unraw_string(&self) -> String;
}

impl UnrawString for syn::Ident {
    fn unraw_string(&self) -> String {
        let s = self.to_string();
        s.strip_prefix("r#").map(String::from).unwrap_or(s)
    }
}

fn style(fields: &syn::Fields) -> &'static str {
    match fields {
        syn::Fields::Named(_) => "named",
        syn::Fields::Unnamed(_) => "tuple",
        syn::Fields::Unit => "unit",
    }
}

/// The derives, the thiserror `#[error]` and the configulator and serde
/// attributes of a struct or enum.
fn type_attrs(v: &mut Map<String, Value>, attrs: &[syn::Attribute]) {
    v.insert("derives".into(), derives(attrs).into());
    if let Some(err) = error_attr(attrs) {
        v.insert("error".into(), err);
    }
    let conf = configulator_attrs(attrs);
    if !conf.is_empty() {
        v.insert("configulator".into(), conf.into());
    }
    if let Some(s) = serde_json_of(attrs) {
        v.insert("serde".into(), s);
    }
}

/// The source text of the node, as written.
fn src<T: ToTokens + Spanned>(node: &T) -> String {
    match node.span().source_text() {
        Some(s) if !s.is_empty() => s,
        _ => node.to_token_stream().to_string(),
    }
}

fn span_json(span: Span, file: &Path) -> Value {
    let (start, end) = (span.start(), span.end());
    json!({
        "file": file.display().to_string(),
        "line": start.line,
        "end_line": end.line,
        "start": span.byte_range().start,
        "end": span.byte_range().end,
    })
}

/// The return type of a signature as written, and "" for none.
fn ret_text(sig: &syn::Signature) -> String {
    match &sig.output {
        syn::ReturnType::Default => String::new(),
        syn::ReturnType::Type(_, ty) => src(&**ty),
    }
}

fn vis_text(vis: &syn::Visibility) -> String {
    match vis {
        syn::Visibility::Inherited => String::new(),
        syn::Visibility::Public(_) => "pub".into(),
        syn::Visibility::Restricted(r) => {
            let path = r.path.to_token_stream().to_string().replace(' ', "");
            if r.in_token.is_some() {
                format!("pub(in {path})")
            } else {
                format!("pub({path})")
            }
        }
    }
}

fn has_attr(attrs: &[syn::Attribute], name: &str) -> bool {
    attrs.iter().any(|a| a.path().is_ident(name))
}

/// The text inside each `#[cfg(...)]`, as written.
fn cfgs(attrs: &[syn::Attribute]) -> Vec<String> {
    attrs
        .iter()
        .filter(|a| a.path().is_ident("cfg"))
        .filter_map(|a| match &a.meta {
            syn::Meta::List(l) => Some(match l.tokens.span().source_text() {
                Some(s) if !s.is_empty() => s,
                _ => l.tokens.to_string(),
            }),
            _ => None,
        })
        .collect()
}

fn path_attr(attrs: &[syn::Attribute]) -> Option<String> {
    attrs
        .iter()
        .find(|a| a.path().is_ident("path"))
        .and_then(|a| match &a.meta {
            syn::Meta::NameValue(nv) => match &nv.value {
                syn::Expr::Lit(syn::ExprLit {
                    lit: syn::Lit::Str(s),
                    ..
                }) => Some(s.value()),
                _ => None,
            },
            _ => None,
        })
}

fn derives(attrs: &[syn::Attribute]) -> Vec<String> {
    let mut out = Vec::new();
    for a in attrs.iter().filter(|a| a.path().is_ident("derive")) {
        let _ = a.parse_nested_meta(|meta| {
            out.push(meta.path.to_token_stream().to_string().replace(' ', ""));
            Ok(())
        });
    }
    out
}

/// The thiserror `#[error(...)]` of an item: its format string, or
/// transparent.
fn error_attr(attrs: &[syn::Attribute]) -> Option<Value> {
    let a = attrs.iter().find(|a| a.path().is_ident("error"))?;
    let syn::Meta::List(l) = &a.meta else {
        return Some(json!({ "text": src(a) }));
    };
    let mut tokens = l.tokens.clone().into_iter();
    match tokens.next() {
        Some(proc_macro2::TokenTree::Ident(id)) if id == "transparent" => {
            Some(json!({ "transparent": true }))
        }
        Some(proc_macro2::TokenTree::Literal(lit)) => {
            let fmt = syn::parse_str::<syn::LitStr>(&lit.to_string())
                .map(|s| s.value())
                .ok();
            let args = tokens.next().is_some();
            Some(json!({ "format": fmt, "args": args, "text": src(&l.tokens) }))
        }
        _ => Some(json!({ "text": src(&l.tokens) })),
    }
}

/// The arguments of each `#[configulator(...)]`, as written.
fn configulator_attrs(attrs: &[syn::Attribute]) -> Vec<String> {
    attrs
        .iter()
        .filter(|a| a.path().is_ident("configulator"))
        .filter_map(|a| match &a.meta {
            syn::Meta::List(l) => Some(src(&l.tokens)),
            _ => None,
        })
        .collect()
}

/// What the `#[serde(...)]` attributes of an item or field say that docgen
/// reads.
#[derive(Default)]
struct SerdeAttrs {
    rename: Option<String>,
    rename_all: Option<String>,
    default: Option<String>,
    skip: bool,
    flatten: bool,
    any: bool,
}

/// Reads the string of `name = "..."`, or of `name(deserialize = "...")`.
fn serde_string(meta: &syn::meta::ParseNestedMeta) -> syn::Result<Option<String>> {
    if meta.input.peek(syn::Token![=]) {
        let s: syn::LitStr = meta.value()?.parse()?;
        return Ok(Some(s.value()));
    }
    let mut out = None;
    meta.parse_nested_meta(|inner| {
        if inner.path.is_ident("deserialize") {
            let s: syn::LitStr = inner.value()?.parse()?;
            out = Some(s.value());
        } else {
            serde_skip(&inner)?;
        }
        Ok(())
    })?;
    Ok(out)
}

/// Reads past the value of a serde argument that docgen does not use.
fn serde_skip(meta: &syn::meta::ParseNestedMeta) -> syn::Result<()> {
    if meta.input.peek(syn::Token![=]) {
        meta.value()?.parse::<syn::Expr>()?;
    } else if meta.input.peek(syn::token::Paren) {
        let content;
        syn::parenthesized!(content in meta.input);
        content.parse::<proc_macro2::TokenStream>()?;
    }
    Ok(())
}

fn serde_attrs(attrs: &[syn::Attribute]) -> SerdeAttrs {
    let mut out = SerdeAttrs::default();
    for a in attrs.iter().filter(|a| a.path().is_ident("serde")) {
        out.any = true;
        let _ = a.parse_nested_meta(|meta| {
            if meta.path.is_ident("rename") {
                if let Some(s) = serde_string(&meta)? {
                    out.rename = Some(s);
                }
            } else if meta.path.is_ident("rename_all") {
                if let Some(s) = serde_string(&meta)? {
                    out.rename_all = Some(s);
                }
            } else if meta.path.is_ident("default") {
                if meta.input.peek(syn::Token![=]) {
                    let s: syn::LitStr = meta.value()?.parse()?;
                    out.default = Some(s.value());
                } else {
                    out.default = Some(String::new());
                }
            } else if meta.path.is_ident("skip") || meta.path.is_ident("skip_deserializing") {
                out.skip = true;
            } else if meta.path.is_ident("flatten") {
                out.flatten = true;
            } else {
                serde_skip(&meta)?;
            }
            Ok(())
        });
    }
    out
}

/// The serde attributes of an item or field as JSON, and None when it has
/// no `#[serde]`.
fn serde_json_of(attrs: &[syn::Attribute]) -> Option<Value> {
    let s = serde_attrs(attrs);
    if !s.any {
        return None;
    }
    Some(json!({
        "rename": s.rename,
        "rename_all": s.rename_all,
        "default": s.default,
        "skip": s.skip,
        "flatten": s.flatten,
    }))
}

/// The `impl Default for Type` of a one-segment type with no generics, with
/// the fields that its `default` fn sets in a struct literal as its tail
/// expression, as written.
fn default_impl(i: &syn::ItemImpl) -> Option<Value> {
    let (_, path, _) = i.trait_.as_ref()?;
    if path.segments.last()?.ident != "Default" || !i.generics.params.is_empty() {
        return None;
    }
    let syn::Type::Path(tp) = &*i.self_ty else {
        return None;
    };
    if tp.qself.is_some() {
        return None;
    }
    let ty = tp.path.segments.last()?.ident.to_string();
    let mut fields = Map::new();
    for ii in &i.items {
        let syn::ImplItem::Fn(f) = ii else {
            continue;
        };
        if f.sig.ident != "default" || f.block.stmts.len() != 1 {
            continue;
        }
        let syn::Stmt::Expr(syn::Expr::Struct(lit), None) = &f.block.stmts[0] else {
            continue;
        };
        for fv in &lit.fields {
            if let (syn::Member::Named(id), Some(_)) = (&fv.member, &fv.colon_token) {
                fields.insert(id.unraw_string(), src(&fv.expr).into());
            }
        }
    }
    Some(json!({ "type": ty, "fields": fields, "cfg": cfgs(&i.attrs) }))
}

/// Each name that a `use` brings into the module: the name, the path it
/// names, and whether it is a glob.
fn use_tree(tree: &syn::UseTree, vis: &str, prefix: &mut Vec<String>, out: &mut Vec<Value>) {
    match tree {
        syn::UseTree::Path(p) => {
            prefix.push(p.ident.to_string());
            use_tree(&p.tree, vis, prefix, out);
            prefix.pop();
        }
        syn::UseTree::Name(n) => {
            let mut path = prefix.clone();
            path.push(n.ident.to_string());
            let name = if n.ident == "self" {
                prefix.last().cloned().unwrap_or_default()
            } else {
                n.ident.to_string()
            };
            if n.ident == "self" {
                path.pop();
            }
            out.push(json!({ "name": name, "path": path, "vis": vis, "glob": false }));
        }
        syn::UseTree::Rename(r) => {
            let mut path = prefix.clone();
            if r.ident != "self" {
                path.push(r.ident.to_string());
            }
            out.push(
                json!({ "name": r.rename.to_string(), "path": path, "vis": vis, "glob": false }),
            );
        }
        syn::UseTree::Glob(_) => {
            out.push(json!({ "name": "*", "path": prefix.clone(), "vis": vis, "glob": true }));
        }
        syn::UseTree::Group(g) => {
            for t in &g.items {
                use_tree(t, vis, prefix, out);
            }
        }
    }
}

#[derive(Clone, Copy, PartialEq)]
enum Style {
    Outer,
    Inner,
    Both,
}

/// The doc of attrs as rustdoc joins it: each `///` line or `#[doc]` value,
/// with the indent that all lines share removed. `#[doc = include_str!("p")]`
/// reads p next to file.
fn doc_of(
    attrs: &[syn::Attribute],
    style: Style,
    file: &Path,
    warnings: &mut Vec<String>,
) -> String {
    let mut lines: Vec<String> = Vec::new();
    let mut included: Vec<(usize, String)> = Vec::new();
    for a in attrs {
        let inner = matches!(a.style, syn::AttrStyle::Inner(_));
        let keep = match style {
            Style::Outer => !inner,
            Style::Inner => inner,
            Style::Both => true,
        };
        if !keep || !a.path().is_ident("doc") {
            continue;
        }
        let syn::Meta::NameValue(nv) = &a.meta else {
            continue;
        };
        match &nv.value {
            syn::Expr::Lit(syn::ExprLit {
                lit: syn::Lit::Str(s),
                ..
            }) => {
                lines.extend(strip_block(&s.value()).lines().map(String::from));
                if s.value().is_empty() {
                    lines.push(String::new());
                }
            }
            syn::Expr::Macro(m) if m.mac.path.is_ident("include_str") => {
                match m.mac.parse_body::<syn::LitStr>() {
                    Ok(p) => {
                        let path = file.parent().unwrap_or(Path::new(".")).join(p.value());
                        match fs::read_to_string(&path) {
                            Ok(text) => included.push((lines.len(), text)),
                            Err(e) => warnings.push(format!("{}: {e}", path.display())),
                        }
                    }
                    Err(_) => warnings.push(format!(
                        "{}: a #[doc] with include_str! of no string literal",
                        file.display()
                    )),
                }
            }
            other => warnings.push(format!(
                "{}:{}: a #[doc] value that is not a string: {}",
                file.display(),
                other.span().start().line,
                src(other)
            )),
        }
    }
    let indent = lines
        .iter()
        .filter(|l| !l.trim().is_empty())
        .map(|l| l.len() - l.trim_start().len())
        .min()
        .unwrap_or(0);
    let mut out: Vec<String> = lines
        .iter()
        .map(|l| {
            if l.trim().is_empty() {
                String::new()
            } else {
                l[indent..].to_string()
            }
        })
        .collect();
    for (at, text) in included.into_iter().rev() {
        out.insert(at, text.trim_end_matches('\n').to_string());
    }
    out.join("\n").trim_matches('\n').to_string()
}

/// The text of a `/** */` block doc with the leading ` * ` of each line
/// removed when every line after the first has one.
fn strip_block(s: &str) -> String {
    if !s.contains('\n') {
        return s.to_string();
    }
    let lines: Vec<&str> = s.lines().collect();
    let starred = lines
        .iter()
        .skip(1)
        .filter(|l| !l.trim().is_empty())
        .all(|l| l.trim_start().starts_with('*'));
    if !starred {
        return s.to_string();
    }
    lines
        .iter()
        .enumerate()
        .map(|(i, l)| {
            let t = l.trim_start();
            if i > 0 && t.starts_with('*') {
                t[1..].to_string()
            } else {
                l.to_string()
            }
        })
        .collect::<Vec<_>>()
        .join("\n")
}

/// The places of the items of one file, for `docgen:code symbol=`: `file`, an
/// `error` for a file that does not parse, `crate_doc` with the line range of
/// each inner doc attribute of the file, and `symbols`. A symbol has its
/// `kind`, its `names` (the first is the one that errors print), its first
/// `line` and `end_line`, which include its attributes and doc comments, the
/// line ranges of its `docs`, and its `cfg`. Items of inline modules are
/// named from the file root, such as `net::connect`; the items of an impl or a
/// trait are named `Type::name`, and the items of a trait impl also
/// `<Type as Trait>::name`.
fn read_symbols(path: &Path) -> Value {
    let file = path.display().to_string();
    match parse(path) {
        Ok((_, parsed)) => {
            let mut out = Vec::new();
            collect_symbols(&parsed.items, &mut Vec::new(), &mut out);
            let crate_doc: Vec<Value> = doc_ranges(&parsed.attrs, usize::MAX)
                .into_iter()
                .map(|(a, b)| json!([a, b]))
                .collect();
            json!({ "file": file, "error": null, "crate_doc": crate_doc, "symbols": out })
        }
        Err(e) => json!({ "file": file, "error": e, "crate_doc": [], "symbols": [] }),
    }
}

/// The line ranges of the doc comments and `#[doc = "..."]` attributes in
/// attrs. An outer one counts when it ends before `ident_line`, the line of
/// the name of the item, so that an item on one line keeps its text; an inner
/// one counts when it starts after that line.
fn doc_ranges(attrs: &[syn::Attribute], ident_line: usize) -> Vec<(usize, usize)> {
    attrs
        .iter()
        .filter(|a| matches!(&a.meta, syn::Meta::NameValue(_)) && a.path().is_ident("doc"))
        .filter_map(|a| {
            let (start, end) = (a.span().start().line, a.span().end().line);
            let inner = matches!(a.style, syn::AttrStyle::Inner(_));
            let keep = if ident_line == usize::MAX {
                inner
            } else if inner {
                start > ident_line
            } else {
                end < ident_line
            };
            keep.then_some((start, end))
        })
        .collect()
}

fn collapse(s: &str) -> String {
    s.split_whitespace().collect::<Vec<_>>().join(" ")
}

fn push_unique(out: &mut Vec<String>, s: String) {
    if !out.contains(&s) {
        out.push(s);
    }
}

fn path_idents(path: &syn::Path) -> String {
    path.segments
        .iter()
        .map(|s| s.ident.unraw_string())
        .collect::<Vec<_>>()
        .join("::")
}

/// The spellings of a type in a symbol: as written, and for a path with no
/// `qself` also without generic arguments and its last name alone.
fn type_forms(ty: &syn::Type) -> Vec<String> {
    let mut out = vec![collapse(&src(ty))];
    if let syn::Type::Path(tp) = ty {
        if tp.qself.is_none() {
            push_unique(&mut out, path_idents(&tp.path));
            if let Some(last) = tp.path.segments.last() {
                push_unique(&mut out, last.ident.unraw_string());
            }
        }
    }
    out
}

/// The spellings of a trait in a symbol: as written, without generic
/// arguments, and the last segment with and without them.
fn trait_forms(path: &syn::Path) -> Vec<String> {
    let mut out = vec![collapse(&src(path))];
    push_unique(&mut out, path_idents(path));
    if let Some(last) = path.segments.last() {
        push_unique(&mut out, collapse(&src(last)));
        push_unique(&mut out, last.ident.unraw_string());
    }
    out
}

struct SymbolAt<'a> {
    kind: &'a str,
    names: Vec<String>,
    attrs: &'a [syn::Attribute],
    whole: Span,
    ident: Span,
}

fn symbol(at: SymbolAt) -> Value {
    let docs: Vec<Value> = doc_ranges(at.attrs, at.ident.start().line)
        .into_iter()
        .map(|(a, b)| json!([a, b]))
        .collect();
    json!({
        "kind": at.kind,
        "name": at.names[0],
        "names": at.names,
        "line": at.whole.start().line,
        "end_line": at.whole.end().line,
        "docs": docs,
        "cfg": cfgs(at.attrs),
    })
}

fn qualified(modpath: &[String], name: &str) -> String {
    if modpath.is_empty() {
        name.to_string()
    } else {
        format!("{}::{name}", modpath.join("::"))
    }
}

fn collect_symbols(items: &[syn::Item], modpath: &mut Vec<String>, out: &mut Vec<Value>) {
    for item in items {
        let whole = item.span();
        let simple = |kind: &'static str, ident: &syn::Ident, attrs: &[syn::Attribute]| {
            symbol(SymbolAt {
                kind,
                names: vec![qualified(modpath, &ident.unraw_string())],
                attrs,
                whole,
                ident: ident.span(),
            })
        };
        match item {
            syn::Item::Const(c) => out.push(simple("const", &c.ident, &c.attrs)),
            syn::Item::Static(s) => out.push(simple("static", &s.ident, &s.attrs)),
            syn::Item::Struct(s) => out.push(simple("struct", &s.ident, &s.attrs)),
            syn::Item::Enum(e) => out.push(simple("enum", &e.ident, &e.attrs)),
            syn::Item::Union(u) => out.push(simple("union", &u.ident, &u.attrs)),
            syn::Item::Fn(f) => out.push(simple("fn", &f.sig.ident, &f.attrs)),
            syn::Item::Type(t) => out.push(simple("type", &t.ident, &t.attrs)),
            syn::Item::Macro(m) => {
                if let Some(ident) = &m.ident {
                    out.push(simple("macro", ident, &m.attrs));
                }
            }
            syn::Item::Trait(t) => {
                out.push(simple("trait", &t.ident, &t.attrs));
                let owner = qualified(modpath, &t.ident.unraw_string());
                for ti in &t.items {
                    let (kind, ident, attrs) = match ti {
                        syn::TraitItem::Fn(f) => ("fn", &f.sig.ident, &f.attrs),
                        syn::TraitItem::Const(c) => ("const", &c.ident, &c.attrs),
                        syn::TraitItem::Type(t) => ("type", &t.ident, &t.attrs),
                        _ => continue,
                    };
                    out.push(symbol(SymbolAt {
                        kind,
                        names: vec![format!("{owner}::{}", ident.unraw_string())],
                        attrs,
                        whole: ti.span(),
                        ident: ident.span(),
                    }));
                }
            }
            syn::Item::Mod(m) => {
                out.push(simple("mod", &m.ident, &m.attrs));
                if let Some((_, inner)) = &m.content {
                    modpath.push(m.ident.unraw_string());
                    collect_symbols(inner, modpath, out);
                    modpath.pop();
                }
            }
            syn::Item::Impl(i) => impl_symbols(i, modpath, out),
            _ => {}
        }
    }
}

/// The impl block, named `impl Type` or `impl Trait for Type`, and its fns,
/// consts and types.
fn impl_symbols(i: &syn::ItemImpl, modpath: &[String], out: &mut Vec<Value>) {
    let selves = type_forms(&i.self_ty);
    let traits: Vec<String> = i
        .trait_
        .as_ref()
        .map_or_else(Vec::new, |(_, p, _)| trait_forms(p));
    let bang = if i.trait_.as_ref().is_some_and(|t| t.0.is_some()) {
        "!"
    } else {
        ""
    };
    let mut names = Vec::new();
    if traits.is_empty() {
        for s in &selves {
            names.push(format!("impl {s}"));
        }
    } else {
        for t in &traits {
            for s in &selves {
                names.push(format!("impl {bang}{t} for {s}"));
            }
        }
    }
    out.push(symbol(SymbolAt {
        kind: "impl",
        names,
        attrs: &i.attrs,
        whole: i.span(),
        ident: i.impl_token.span,
    }));
    for ii in &i.items {
        let (kind, ident, attrs) = match ii {
            syn::ImplItem::Fn(f) => ("fn", &f.sig.ident, &f.attrs),
            syn::ImplItem::Const(c) => ("const", &c.ident, &c.attrs),
            syn::ImplItem::Type(t) => ("type", &t.ident, &t.attrs),
            _ => continue,
        };
        let name = ident.unraw_string();
        let mut names = Vec::new();
        for t in &traits {
            for s in &selves {
                names.push(qualified(modpath, &format!("<{s} as {t}>::{name}")));
            }
        }
        for s in &selves {
            names.push(qualified(modpath, &format!("{s}::{name}")));
        }
        out.push(symbol(SymbolAt {
            kind,
            names,
            attrs,
            whole: ii.span(),
            ident: ident.span(),
        }));
    }
}
