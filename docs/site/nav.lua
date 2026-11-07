local stringify = pandoc.utils.stringify

local M = {}

M.docs = os.getenv("DOCS_DIR") or "docs"
M.site = os.getenv("DOCS_SITE") or (M.docs .. "/site")
M.nav = M.docs .. "/nav.md"

-- in_docs returns the path of p in the docs directory, or nil when p is
-- outside it.
function M.in_docs(p)
  local prefix = M.docs .. "/"
  if p:sub(1, #prefix) == prefix then
    return p:sub(#prefix + 1)
  end
end

-- fail ends the build with a message and no Lua traceback.
function M.fail(msg)
  io.stderr:write("build.sh: " .. msg .. "\n")
  os.exit(1)
end

function M.read_file(p)
  local f = io.open(p, "rb")
  if not f then
    return nil
  end
  local data = f:read("a")
  f:close()
  return data
end

function M.is_dir(p)
  return pcall(pandoc.system.list_directory, p)
end

function M.resolve(dir, target)
  local parts = {}
  local whole = dir == "" and target or dir .. "/" .. target
  for seg in whole:gmatch("[^/]+") do
    if seg == ".." then
      if #parts == 0 then
        return nil
      end
      table.remove(parts)
    elseif seg ~= "." then
      table.insert(parts, seg)
    end
  end
  return table.concat(parts, "/")
end

function M.relative(from_dir, to)
  local a, b = {}, {}
  for s in from_dir:gmatch("[^/]+") do
    table.insert(a, s)
  end
  for s in to:gmatch("[^/]+") do
    table.insert(b, s)
  end
  local i = 1
  while i <= #a and i < #b and a[i] == b[i] do
    i = i + 1
  end
  local out = {}
  for _ = i, #a do
    table.insert(out, "..")
  end
  for j = i, #b do
    table.insert(out, b[j])
  end
  return table.concat(out, "/")
end

function M.dir_of(p)
  return p:match("^(.*)/[^/]*$") or ""
end

function M.default_out(src)
  if src == "README.md" then
    return "index.html"
  end
  local rest = M.in_docs(src)
  if not rest then
    return nil
  end
  local d = rest:match("^(.*)/README%.md$")
  if d then
    return d .. ".html"
  end
  local p = rest:match("^(.*)%.md$")
  return p and p .. ".html" or nil
end

local function item_of(blocks)
  local plain = blocks[1]
  if not plain or (plain.t ~= "Plain" and plain.t ~= "Para") then
    M.fail(M.nav .. ": an item has no text")
  end
  local link, code, old
  for _, x in ipairs(plain.content) do
    if x.t == "Link" and not link then
      link = x
    elseif x.t == "Code" and not code then
      code = x
    elseif x.t == "RawInline" and x.format == "html" then
      old = old or x.text:match("^<!%-%-%s*old:%s*(%S+)%s*%-%->$")
    end
  end
  local it = {}
  if link then
    it.title = stringify(link.content)
    it.target = link.target
  elseif code then
    local t = {}
    for _, x in ipairs(plain.content) do
      if x.t == "Code" then
        break
      end
      table.insert(t, x)
    end
    it.title = stringify(t):gsub("[:%s]+$", "")
    it.planned = code.text
  else
    it.title = stringify(plain.content)
  end
  it.old = old
  return it
end

-- load reads nav.md of the docs directory and returns the sections, each with a title and
-- items, and a map from each source file to its output page. It runs from
-- the repository root.
function M.load()
  if not M.read_file("README.md") then
    M.fail("README.md not found at the repository root: it is the front page of the site")
  end
  local text = M.read_file(M.nav)
  if not text then
    M.fail(M.nav .. " not found")
  end
  local doc = pandoc.read(text, "gfm")
  local list
  for _, b in ipairs(doc.blocks) do
    if b.t == "BulletList" then
      list = b
      break
    end
  end
  if not list then
    M.fail(M.nav .. " has no list")
  end
  local nav = { sections = {}, pages = {}, by_src = {}, by_out = {}, missing = {} }
  local function add(it, section)
    it.section = section
    if it.planned then
      if not M.read_file(M.docs .. "/" .. it.planned) then
        it.kind = "planned"
        it.out = M.default_out(M.docs .. "/" .. it.planned)
        table.insert(nav.missing, it)
        return it
      end
      io.stderr:write("written: " .. M.docs .. "/" .. it.planned .. "; make it a link in " .. M.nav .. "\n")
      it.target = it.planned
    end
    if not it.target then
      M.fail(M.nav .. ": item " .. it.title .. " has no link and no path")
    end
    if it.target:match("^%a[%w+.-]*:") then
      it.kind = "external"
      return it
    end
    local p = M.resolve(M.docs, it.target)
    local out = p and M.default_out(p)
    if not out or not p:match("%.md$") then
      it.kind = "file"
      it.path = p
      return it
    end
    it.out = out
    local src = p
    if not M.read_file(src) and it.old then
      local o = M.resolve(M.docs, it.old)
      if o and M.read_file(o) then
        src = o
        it.moved = o
      end
    end
    if not M.read_file(src) then
      it.kind = "planned"
      it.planned = M.in_docs(p)
      table.insert(nav.missing, it)
      return it
    end
    it.kind = "page"
    it.src = src
    nav.by_src[src] = out
    nav.by_src[p] = out
    nav.by_out[out] = it
    table.insert(nav.pages, it)
    return it
  end
  for _, entry in ipairs(list.content) do
    local head = item_of(entry)
    local sub
    for _, b in ipairs(entry) do
      if b.t == "BulletList" then
        sub = b
      end
    end
    local section = { title = head.title, items = {} }
    if sub then
      for _, child in ipairs(sub.content) do
        table.insert(section.items, add(item_of(child), section))
      end
    else
      section.title = nil
      table.insert(section.items, add(head, section))
    end
    table.insert(nav.sections, section)
  end
  return nav
end

-- extra returns the Markdown files of the docs directory that nav.md does not list,
-- other than nav.md, in path order.
function M.extra(nav)
  local out = {}
  local function walk(dir)
    for _, name in ipairs(pandoc.system.list_directory(dir)) do
      local p = dir .. "/" .. name
      if M.is_dir(p) then
        if p ~= M.site then
          walk(p)
        end
      elseif name:match("%.md$") and p ~= M.nav and not nav.by_src[p] then
        table.insert(out, p)
      end
    end
  end
  walk(M.docs)
  table.sort(out)
  return out
end

-- built returns a map from the source file of each page of the site to its
-- output page: the pages of nav.md and the other Markdown files that pages.lua
-- builds.
function M.built(nav)
  local out = {}
  for src, page in pairs(nav.by_src) do
    out[src] = page
  end
  for _, p in ipairs(M.extra(nav)) do
    local page = M.default_out(p)
    if not nav.by_out[page] then
      out[p] = page
    end
  end
  return out
end

-- module_path returns the path of the module directive of the go.mod in dir,
-- without quotes. It returns nil when the file has no such directive, and also
-- false when there is no file.
local function module_path(dir)
  local text = M.read_file(dir .. "/go.mod")
  if not text then
    return nil, false
  end
  local block = false
  for line in (text .. "\n"):gmatch("(.-)\r?\n") do
    line = line:gsub("//.*$", ""):match("^%s*(.-)%s*$")
    local path
    if block then
      path = line ~= "" and line ~= ")" and line
    elseif line:match("^module%s*%($") then
      block = true
    else
      path = line:match("^module%s+(.+)$")
    end
    if path then
      return (path:gsub("^[\"`]", ""):gsub("[\"`]$", ""))
    end
  end
  return nil, true
end

local function module_dir()
  return ((os.getenv("DOCS_MODULE") or "."):gsub("^%./", ""):gsub("/$", ""))
end

local function without_major(mod)
  return (mod:gsub("/v%d+$", ""))
end

-- required_module returns the module path of the go.mod of DOCS_MODULE, and
-- ends the build when there is none.
local function required_module()
  local dir = module_dir()
  local mod, file = module_path(dir)
  if mod then
    return mod, dir
  end
  if file then
    M.fail(dir .. "/go.mod has no module line")
  end
  if os.getenv("DOCS_MODULE") then
    M.fail("no go.mod in DOCS_MODULE directory " .. dir)
  end
  M.fail("no go.mod at the repository root; set DOCS_MODULE to the directory of the Go module")
end

-- repo_url returns the repository URL: the module path of the go.mod at the
-- repository root, or of the go.mod in DOCS_MODULE without that directory,
-- and in both cases without a major version suffix.
function M.repo_url()
  local mod = module_path(".")
  if mod then
    return "https://" .. without_major(mod)
  end
  local dir
  mod, dir = required_module()
  mod = without_major(mod)
  if mod:sub(-#dir - 1) == "/" .. dir then
    mod = mod:sub(1, -#dir - 2)
  end
  return "https://" .. mod
end

function M.go_api_url()
  return "https://pkg.go.dev/" .. required_module()
end

-- pin_api returns url with @tag after the module path when url is a pkg.go.dev
-- link of this module, and url as it is in other cases. A path that continues
-- with a major version, such as /v2, is a different module.
function M.pin_api(url, tag)
  local base = M.go_api_url()
  if url:sub(1, #base) ~= base then
    return url
  end
  local rest = url:sub(#base + 1)
  if rest ~= "" and not rest:match("^[/#?]") then
    return url
  end
  if rest:match("^/v%d+$") or rest:match("^/v%d+[/#?]") then
    return url
  end
  return base .. "@" .. tag .. rest
end

return M
