package.path = (os.getenv("DOCS_SITE") or "docs/site") .. "/?.lua;" .. package.path
local nav = require("nav")

local n = nav.load()
nav.repo_url()
nav.go_api_url()
for _, it in ipairs(n.missing) do
  io.stderr:write("not yet written: " .. nav.docs .. "/" .. it.planned .. " (" .. it.title .. ")\n")
end
for _, it in ipairs(n.pages) do
  if it.moved then
    io.stderr:write("old path: " .. it.out .. " comes from " .. it.moved .. " until " .. nav.docs .. "/" .. it.target .. " exists\n")
  end
  print(it.src .. "\t" .. it.out)
end
for _, p in ipairs(nav.extra(n)) do
  local out = nav.default_out(p)
  if n.by_out[out] then
    io.stderr:write("not in " .. nav.nav .. ": " .. p .. " (skipped: " .. out .. " is taken)\n")
  else
    io.stderr:write("not in " .. nav.nav .. ": " .. p .. "\n")
    print(p .. "\t" .. out)
  end
end
