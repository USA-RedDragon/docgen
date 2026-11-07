package.path = (os.getenv("DOCS_SITE") or "docs/site") .. "/?.lua;" .. package.path
local navlib = require("nav")
local stringify = pandoc.utils.stringify
local read_file, resolve, relative, dir_of = navlib.read_file, navlib.resolve, navlib.relative, navlib.dir_of

local function html(s)
  return (s:gsub("&", "&amp;"):gsub("<", "&lt;"):gsub(">", "&gt;"):gsub('"', "&quot;"))
end

local function first_header(blocks, level)
  for _, b in ipairs(blocks) do
    if b.t == "Header" and b.level == level then
      return b
    end
  end
end

local function only_images(inlines)
  local n = 0
  for _, x in ipairs(inlines) do
    if x.t == "Image" then
      n = n + 1
    elseif x.t ~= "Space" and x.t ~= "SoftBreak" and x.t ~= "LineBreak" then
      return false
    end
  end
  return n > 0
end

local function blank(inlines)
  for _, x in ipairs(inlines) do
    if x.t ~= "Space" and x.t ~= "SoftBreak" and x.t ~= "LineBreak" then
      return false
    end
  end
  return true
end

local function out_path(p)
  return navlib.in_docs(p) or p
end

local function write_file(p, data)
  pandoc.system.make_directory(dir_of(p), true)
  local f = assert(io.open(p, "wb"))
  f:write(data)
  f:close()
end

-- source_url returns the link of the file or directory p of the repository at
-- ref.
local function source_url(repo, ref, p)
  return repo .. (navlib.is_dir(p) and "/tree/" or "/blob/") .. ref .. "/" .. p
end

-- banner_html returns the text of the banner of a version that is not the
-- latest. latest is nil when there are no tags, and pages is the page list of
-- the latest version.
local function banner_html(version, latest, pages, page, root)
  if not version or version == latest then
    return nil
  end
  local text = html(version) .. " is not the latest version."
  if version == "main" then
    text = "<strong>Not released.</strong> These pages come from the main branch."
  end
  if not latest then
    return text
  end
  local same = ("\n" .. pages):find("\t" .. page .. "\n", 1, true)
  local href = root .. "../latest/" .. (same and page or "")
  return text .. ' <a href="' .. html(href) .. '">' .. (same and "Read this page in the latest version" or "Go to the latest version") .. "</a>."
end

local function toc_html(toc)
  if #toc < 2 then
    return nil
  end
  local items = {}
  for _, t in ipairs(toc) do
    table.insert(items, '<li class="l' .. t.level .. '"><a href="#' .. html(t.id) .. '">' .. html(t.title) .. "</a></li>")
  end
  return "<ul>" .. table.concat(items) .. "</ul>"
end

-- nav_html returns the navigation. toc is the list of the headings of the page, or
-- nil, and goes under the item of the page, in a closed native disclosure, so
-- that it works with no script. A page with no item gets a section for the list.
local function nav_html(nav, page, root, repo, ref, pin, toc)
  local parts, placed = {}, false
  for _, sec in ipairs(nav.sections) do
    local items = {}
    for _, it in ipairs(sec.items) do
      local href, attrs = nil, ""
      if it.kind == "page" then
        href = root .. it.out
        if it.out == page then
          attrs = ' class="on" aria-current="page"'
        end
      elseif it.kind == "external" then
        href = pin(it.target)
      elseif it.kind == "file" and it.path and (read_file(it.path) or navlib.is_dir(it.path)) then
        href = source_url(repo, ref, it.path)
      end
      if href then
        local link = '<a href="' .. html(href) .. '"' .. attrs .. ">" .. html(it.title) .. "</a>"
        if toc and it.kind == "page" and it.out == page and not placed then
          placed = true
          table.insert(items, '<li class="here">' .. link .. '<details class="toc"><summary aria-label="On this page" title="On this page"></summary>' .. toc .. "</details></li>")
        else
          table.insert(items, "<li>" .. link .. "</li>")
        end
      end
    end
    if #items > 0 then
      if sec.title then
        table.insert(parts, '<li class="sec"><span>' .. html(sec.title) .. "</span><ul>" .. table.concat(items) .. "</ul></li>")
      else
        table.insert(parts, table.concat(items))
      end
    end
  end
  if toc and not placed then
    table.insert(parts, '<li class="sec"><span>On this page</span>' .. toc .. "</li>")
  end
  return "<ul>" .. table.concat(parts) .. "</ul>"
end

local function pager_html(nav, page, root)
  local idx
  for i, it in ipairs(nav.pages) do
    if it.out == page then
      idx = i
    end
  end
  if not idx then
    return nil
  end
  local out = {}
  local prev, nxt = nav.pages[idx - 1], nav.pages[idx + 1]
  if prev then
    table.insert(out, '<a class="prev" rel="prev" href="' .. html(root .. prev.out) .. '"><span>Previous</span>' .. html(prev.title) .. "</a>")
  end
  if nxt then
    table.insert(out, '<a class="next" rel="next" href="' .. html(root .. nxt.out) .. '"><span>Next</span>' .. html(nxt.title) .. "</a>")
  end
  if #out == 0 then
    return nil
  end
  return table.concat(out)
end

local function plain_text(blocks, limit)
  local s = stringify(pandoc.Pandoc(blocks)):gsub("%s+", " ")
  if #s > limit then
    s = s:sub(1, limit)
  end
  return s
end

-- without_source returns blocks with no excerpt of an editor block, so that the
-- search text of a page holds its prose.
local function without_source(blocks)
  return blocks:walk({
    Div = function(d)
      if d.classes:includes("editor-source") then
        return {}
      end
    end,
  })
end

local function is_mark(b, text)
  return b.t == "RawBlock" and b.text:match("^%s*<!%-%-%s*" .. text .. "%s*%-%->%s*$")
end

local function first_code(blocks)
  for _, b in ipairs(blocks) do
    if b.t == "CodeBlock" then
      return b.text
    end
  end
end

-- rendered returns the blocks of Markdown text, with its images resolved from
-- dir and its headings kept out of the table of contents.
local function rendered(text, dir)
  return pandoc.read(text, "gfm").blocks:walk({
    Image = function(img)
      if not img.src:match("^%a[%w+.-]*:") then
        img.src = dir == "" and img.src or dir .. "/" .. img.src
      end
      return img
    end,
    Header = function(h)
      h.classes:insert("sample")
      h.identifier = ""
      return h
    end,
  })
end

-- marker_args returns the key=value arguments of the text of an open marker, by
-- the rules of docgen: a value holds no white space unless it is in double
-- quotes.
local function marker_args(text)
  local args = {}
  local rest = text:gsub('(%l+)="([^"]*)"', function(k, v)
    args[k] = v
    return ""
  end)
  for k, v in rest:gmatch('(%l+)=([^%s>"]+)') do
    args[k] = v
  end
  return args
end

-- open_args returns the arguments of the block b when it is the open marker of
-- the given name. It returns nil for any other block.
local function open_args(b, name)
  local text = b.t == "RawBlock" and b.text:match("^%s*<!%-%-%s*" .. name .. "%s+(.-)%s*%-%->%s*$")
  return text and marker_args(text)
end

-- docgen_dir returns the directory of the page of the docgen:docgen marker args,
-- from the directory here of the page that holds the marker.
local function docgen_dir(args, here)
  local dir = resolve("", (args.root or "") .. "/" .. dir_of(args.page or ""))
  return dir and relative(here, dir .. "/x"):gsub("/?x$", "") or ""
end

-- docgen_panels returns the panels of the body of a docgen:docgen marker, for the
-- blocks from index i up to the close mark. Each heading at the level of the
-- first one starts a panel, and the code block of the last panel, as Markdown, is
-- one more panel named Preview, with its images resolved from dir. It also
-- returns the index of the close mark.
local function docgen_panels(blocks, i, dir)
  local panels, level = {}, nil
  while i <= #blocks and not is_mark(blocks[i], "/docgen:docgen") do
    local b = blocks[i]
    if b.t == "Header" and (not level or b.level == level) then
      level = b.level
      table.insert(panels, { title = stringify(b), blocks = pandoc.Blocks({}) })
    elseif #panels > 0 then
      panels[#panels].blocks:insert(b)
    end
    i = i + 1
  end
  local text = #panels > 0 and first_code(panels[#panels].blocks)
  if text then
    table.insert(panels, { title = "Preview", blocks = rendered(text, dir), page = true })
  end
  return panels, i
end

-- expand_tabs returns s with each tab as spaces up to a column of 4, which is
-- what pandoc does to the text of a page before it parses it.
local function expand_tabs(s)
  return (s:gsub("[^\n]+", function(line)
    local at = line:find("\t", 1, true)
    while at do
      line = line:sub(1, at - 1) .. (" "):rep(4 - (at - 1) % 4) .. line:sub(at + 1)
      at = line:find("\t", at, true)
    end
    return line
  end))
end

-- first_line returns the line of the file p where text starts, when text is
-- whole lines of the file and occurs once. It returns nil in any other case, so
-- that a page shows no line number that is not true. The second result is a line
-- to sort by: the same line, or else the first line that equals the first line of
-- text, or else math.huge.
local function first_line(p, text)
  local data = p and read_file(p)
  if not data then
    return nil, math.huge
  end
  data = "\n" .. expand_tabs(data:gsub("\r\n", "\n")) .. "\n"
  local lines = "\n" .. text .. "\n"
  local at = data:find(lines, 1, true)
  local once = at and not data:find(lines, at + 1, true)
  at = at or data:find("\n" .. text:match("^[^\n]*") .. "\n", 1, true)
  if not at then
    return nil, math.huge
  end
  local line = select(2, data:sub(1, at):gsub("\n", ""))
  return once and line or nil, line
end

-- in_root returns the path p without the directory root, or p when p is outside
-- root.
local function in_root(p, root)
  if root and root ~= "" and p:sub(1, #root + 1) == root .. "/" then
    return p:sub(#root + 2)
  end
  return p
end

-- plain_version returns one version of a file of an editor block. name is nil
-- for a file with one version.
local function plain_version(name, code, class)
  local blocks = pandoc.Blocks({})
  if name then
    blocks:insert(pandoc.Header(5, name, { class = "sample" }))
  end
  return pandoc.Div(blocks .. code, { class = class, ["data-name"] = name })
end

-- editor_file returns the block of the file f of an editor block: a heading with
-- its path, then its versions as plain blocks. docs/site/editor.js reads this
-- block, and it is what a reader sees with no JavaScript. The path of a page of
-- a docgen:docgen marker is relative to its root already.
local function editor_file(f, root)
  local path = f.whole and f.path or in_root(f.path, root)
  local blocks = pandoc.Blocks({ pandoc.Header(4, { pandoc.Code(path) }, { class = "sample" }) })
  blocks:extend(f.blocks)
  return pandoc.Div(blocks, { class = "editor-file", ["data-path"] = path, ["data-line"] = f.line and tostring(f.line) or nil })
end

-- editor_block returns the block of the given files. Any path shows relative to
-- root.
local function editor_block(files, root, large)
  local out = pandoc.Blocks({})
  for _, f in ipairs(files) do
    out:insert(editor_file(f, root))
  end
  return pandoc.Div(out, { class = large and "editor large" or "editor", ["data-root"] = root and root:match("[^/]*$") or nil })
end

-- docgen_file returns the file of the docgen:docgen marker with arguments args at
-- index i, its root, and the index of the close mark.
local function docgen_file(blocks, i, args, here, fail)
  local panels
  panels, i = docgen_panels(blocks, i + 1, docgen_dir(args, here))
  local out = pandoc.Blocks({})
  for _, p in ipairs(panels) do
    if not p.page and not first_code(p.blocks) then
      fail("has a tab with no code: " .. p.title)
    end
    out:insert(plain_version(p.title, p.blocks, p.page and "editor-preview" or "editor-version"))
  end
  if #out == 0 or not args.page then
    fail("has a docgen:docgen marker with no page= or no code; run docgen update")
  end
  -- A docgen:docgen body is the whole page, so its lines start at 1.
  return { path = resolve("", args.page) or args.page, line = 1, blocks = out, whole = true }, resolve("", args.root or "") or "", i
end

-- code_blocks returns the version of the file f of docgen:code markers, with the
-- excerpts in file order. A true line number goes on the block of its excerpt.
local function code_blocks(f, fail)
  table.sort(f.parts, function(a, b)
    if a.order ~= b.order then
      return a.order < b.order
    end
    return a.index < b.index
  end)
  local out, last = pandoc.Blocks({}), 0
  for _, part in ipairs(f.parts) do
    if part.line then
      if part.line <= last then
        fail("has excerpts of " .. f.path .. " that overlap")
      end
      last = part.line + select(2, part.text:gsub("\n", ""))
    end
    out:insert(pandoc.Div(part.body, { class = "editor-source", ["data-line"] = part.line and tostring(part.line) or nil }))
  end
  return pandoc.Blocks({ plain_version(nil, out, "editor-version") })
end

-- editor returns one editor block for the blocks after the open mark at index i,
-- up to the close mark, and the index of the close mark. The block can hold
-- docgen:code and docgen:docgen markers only, and the excerpts of one file make
-- one file of the block. Any path shows relative to the root of the first
-- docgen:docgen marker of the block.
local function editor(blocks, i, here, src, large)
  local files, by_path, root = {}, {}, nil
  local function fail(msg)
    navlib.fail(src .. ": site:editor " .. msg)
  end
  i = i + 1
  while i <= #blocks and not is_mark(blocks[i], "/site:editor") do
    local b = blocks[i]
    local code, page = open_args(b, "docgen:code"), open_args(b, "docgen:docgen")
    if code then
      local start = i
      while i <= #blocks and not is_mark(blocks[i], "/docgen:code") do
        i = i + 1
      end
      local body = pandoc.Blocks({})
      for n = start + 1, math.min(i - 1, #blocks) do
        body:insert(blocks[n])
      end
      local text = first_code(body)
      if not code.file or not text then
        fail("has a docgen:code marker with no file= or no code; run docgen update")
      end
      local p = resolve("", code.file)
      local path = p or code.file
      local f = by_path[path]
      if not f then
        f = { path = path, parts = {} }
        by_path[path] = f
        table.insert(files, f)
      end
      local line, order = first_line(p, text)
      table.insert(f.parts, { line = line, order = order, index = #f.parts, text = text, body = body })
    elseif page then
      local f, top
      f, top, i = docgen_file(blocks, i, page, here, fail)
      root = root or top
      table.insert(files, f)
    elseif b.t ~= "RawBlock" or not b.text:match("^%s*<!%-%-.*%-%->%s*$") then
      fail("can hold docgen:code and docgen:docgen markers only")
    end
    i = i + 1
  end
  if i > #blocks then
    fail("has no <!-- /site:editor -->")
  end
  if #files == 0 then
    fail("holds no files")
  end
  for _, f in ipairs(files) do
    if f.parts then
      f.blocks = code_blocks(f, fail)
    end
  end
  return editor_block(files, root, large), i
end

-- editors replaces each <!-- site:editor --> and <!-- /site:editor --> pair with
-- one editor block, and each <!-- docgen:docgen ... --> and <!-- /docgen:docgen -->
-- pair outside such a block with an editor block of that one page. Images resolve
-- from the directory of the page of the marker. A <!-- site:large --> before a
-- block makes it large. The second result is true when the page holds an editor
-- block.
local function editors(blocks, here, src)
  local out, i, large, found = pandoc.Blocks({}), 1, false, false
  while i <= #blocks do
    local b = blocks[i]
    local args = open_args(b, "docgen:docgen")
    if is_mark(b, "site:large") then
      large = true
    elseif is_mark(b, "site:editor") then
      local div
      div, i = editor(blocks, i, here, src, large)
      out:insert(div)
      large, found = false, true
    elseif args then
      local f, top
      f, top, i = docgen_file(blocks, i, args, here, function(msg)
        navlib.fail(src .. ": " .. msg)
      end)
      out:insert(editor_block({ f }, top, large))
      large, found = false, true
    else
      out:insert(b)
      large = false
    end
    i = i + 1
  end
  return out, found
end

function Pandoc(doc)
  local src = PANDOC_STATE.input_files[1]
  local here = dir_of(src)
  local outdir = stringify(doc.meta.outdir)
  local page = stringify(doc.meta.page)
  local page_dir = dir_of(page)
  local root = relative(page_dir, "x"):gsub("x$", "")
  local repo = navlib.repo_url()
  local nav = navlib.load()
  local built = navlib.built(nav)
  local readme = src == "README.md" and doc or pandoc.read(read_file("README.md"), "gfm")
  local version = doc.meta.version and stringify(doc.meta.version)
  local latest = doc.meta.latest and stringify(doc.meta.latest)
  local latest_pages = doc.meta.latestpages and read_file(stringify(doc.meta.latestpages))
  if version then
    local sha = doc.meta.mainsha and stringify(doc.meta.mainsha) or ""
    if version == "main" and sha ~= "" then
      doc.meta.versionlabel = "main (" .. sha .. ")"
    elseif version == latest then
      doc.meta.versionlabel = version .. " (latest)"
    else
      doc.meta.versionlabel = version
    end
  end
  local ref = version or "main"
  local function pin(url)
    if ref == "main" then
      return url
    end
    return navlib.pin_api(url, ref)
  end

  local has_editor
  doc.blocks, has_editor = editors(doc.blocks, here, src)
  if has_editor then
    write_file(outdir .. "/.editor", "")
  end

  doc = doc:walk({
    Image = function(img)
      if img.src:match("^%a[%w+.-]*:") then
        return {}
      end
      local p = resolve(here, img.src)
      local data = p and read_file(p)
      if not data then
        navlib.fail(src .. ": image " .. img.src .. " not found")
      end
      write_file(outdir .. "/" .. out_path(p), data)
      img.src = relative(page_dir, out_path(p))
      return img
    end,
    Link = function(l)
      if #l.content == 0 then
        return {}
      end
      local t = l.target
      if t:match("^%a[%w+.-]*:") then
        l.target = pin(t)
        return l
      end
      if t:match("^#") then
        return l
      end
      local file, frag = t:match("^([^#]*)(#?.*)$")
      local p = resolve(here, file)
      if not p then
        return l
      end
      local target = built[p]
      if target then
        l.target = relative(page_dir, target) .. frag
      elseif read_file(p) or navlib.is_dir(p) then
        l.target = source_url(repo, ref, p) .. frag
      end
      return l
    end,
  })

  local toc, heads = {}, {}
  local h1
  doc = doc:walk({
    Para = function(para)
      if blank(para.content) then
        return {}
      end
      if only_images(para.content) then
        return pandoc.Div(pandoc.Plain(para.content), { class = "figs" })
      end
    end,
    Table = function(tbl)
      return pandoc.Div({ tbl }, { class = "table" })
    end,
    Header = function(h)
      if h.classes:includes("sample") then
        return h
      end
      local title = stringify(h)
      if h.level == 1 then
        h1 = h1 or title
        return h
      end
      if h.level <= 3 then
        table.insert(toc, { id = h.identifier, title = title, level = h.level })
      end
      table.insert(heads, { h.identifier, title })
      if h.identifier ~= "" then
        h.content:insert(pandoc.Link({ pandoc.Str("§") }, "#" .. h.identifier, "", { class = "self", ["aria-label"] = "Link to " .. title }))
      end
      return h
    end,
  })

  local site = stringify(first_header(readme.blocks, 1) or "")
  local description
  for i, b in ipairs(doc.blocks) do
    if b.t == "Para" then
      description = stringify(b)
      if page == "index.html" then
        doc.blocks[i] = pandoc.Div({ b }, { class = "lede" })
      end
      break
    end
  end

  local section
  local it = nav.by_out[page]
  if it and it.section then
    section = it.section.title
  end
  local title = h1 or site
  local entry = { u = page, t = page == "index.html" and site or title, s = section or "", h = heads, x = plain_text(without_source(doc.blocks), 6000) }
  write_file(outdir .. "/.search/" .. page:gsub("[%%/]", { ["%"] = "%25", ["/"] = "%2F" }) .. ".json", pandoc.json.encode(entry))

  local raw = function(s) return pandoc.MetaBlocks({ pandoc.RawBlock("html", s) }) end
  local links = { '<a href="' .. html(pin(navlib.go_api_url())) .. '">Go API</a>', '<a href="' .. repo .. '">GitHub</a>' }
  doc.meta.outdir = nil
  doc.meta.page = nil
  doc.meta.latest = nil
  doc.meta.latestpages = nil
  doc.meta.pagepath = page
  doc.meta.root = root
  doc.meta.site = site
  doc.meta.pagetitle = (page == "index.html" or not h1) and site or h1 .. " - " .. site
  doc.meta.description = description or site
  doc.meta.nav = raw(nav_html(nav, page, root, repo, ref, pin, toc_html(toc)))
  doc.meta.links = raw(table.concat(links))
  local pager = pager_html(nav, page, root)
  doc.meta.pager = pager and raw(pager) or nil
  doc.meta.section = section
  local banner = banner_html(version, latest, latest_pages, page, root)
  doc.meta.banner = banner and raw(banner) or nil
  doc.meta.source = repo .. "/blob/" .. ref .. "/" .. src
  doc.meta.srcname = src
  doc.meta.home = page == "index.html" and "true" or nil
  doc.meta.editor = has_editor and "true" or nil
  return doc
end
