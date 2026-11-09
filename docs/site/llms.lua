-- llms.lua OUT [PREFIX] writes OUT/llms.txt, the list of the Markdown copies of
-- the pages in the order of nav.md, in the llms.txt format. PREFIX goes before
-- each link, for a list at the top of the site that points into a version.
package.path = (os.getenv("DOCS_SITE") or "docs/site") .. "/?.lua;" .. package.path
local navlib = require("nav")

local outdir = arg[1]
local prefix = arg[2] or ""
local nav = navlib.load()

local readme = pandoc.read(navlib.read_file("README.md"), "gfm")
local title, summary
for _, b in ipairs(readme.blocks) do
  if not title and b.t == "Header" and b.level == 1 then
    title = pandoc.utils.stringify(b.content)
  elseif title and b.t == "Para" then
    -- A row of badges is images in links, with no text of its own.
    for _, inline in ipairs(b.content) do
      if inline.t == "Str" then
        summary = pandoc.utils.stringify(b.content)
        break
      end
    end
    if summary then
      break
    end
  end
end

local lines = { "# " .. (title or "Docs"), "" }
if summary then
  table.insert(lines, "> " .. summary)
  table.insert(lines, "")
end
table.insert(lines, "Each link is the Markdown source of a page of the site.")
for _, section in ipairs(nav.sections) do
  local items = {}
  for _, it in ipairs(section.items) do
    if it.kind == "page" then
      table.insert(items, "- [" .. it.title .. "](" .. prefix .. it.out:gsub("%.html$", ".md") .. ")")
    end
  end
  if #items > 0 then
    table.insert(lines, "")
    table.insert(lines, "## " .. (section.title or "Overview"))
    table.insert(lines, "")
    for _, line in ipairs(items) do
      table.insert(lines, line)
    end
  end
end

local f = assert(io.open(outdir .. "/llms.txt", "wb"))
f:write(table.concat(lines, "\n") .. "\n")
f:close()
