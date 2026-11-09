-- versions.lua holds the version logic of build.sh --versions. It takes a
-- command and tags:
--
--   order TAG...     prints the semver tags, newest first
--   latest TAG...    prints the latest of the tags, which are in order
--   site OUT TAG...  writes versions.json and the redirect at the top of OUT

local function numeric(id)
  return id:match("^%d+$") ~= nil
end

-- parse returns the core numbers and the prerelease identifiers of tag, or nil
-- when tag is not v and a semantic version.
local function parse(tag)
  local a, b, c, rest = tag:match("^v(%d+)%.(%d+)%.(%d+)(.*)$")
  if not a then
    return nil
  end
  for _, n in ipairs({ a, b, c }) do
    if #n > 1 and n:sub(1, 1) == "0" then
      return nil
    end
  end
  local pre, meta = rest:match("^%-([^+]*)(.*)$")
  if not pre then
    pre, meta = "", rest
  elseif pre == "" then
    return nil
  end
  if meta ~= "" and not meta:match("^%+[%w.-]+$") then
    return nil
  end
  local ids = {}
  for id in (pre .. "."):gmatch("([^.]*)%.") do
    if pre ~= "" then
      if not id:match("^[%w-]+$") or (numeric(id) and #id > 1 and id:sub(1, 1) == "0") then
        return nil
      end
      table.insert(ids, id)
    end
  end
  return { core = { a, b, c }, pre = ids, tag = tag }
end

-- compare_numbers compares two digit strings with no leading zeros, of any
-- length, and returns a negative number, zero or a positive number.
local function compare_numbers(x, y)
  if #x ~= #y then
    return #x - #y
  end
  return x == y and 0 or (x < y and -1 or 1)
end

local function compare_ids(x, y)
  local nx, ny = numeric(x), numeric(y)
  if nx and ny then
    return compare_numbers(x, y)
  end
  if nx ~= ny then
    return nx and -1 or 1
  end
  return x == y and 0 or (x < y and -1 or 1)
end

local function compare(x, y)
  for i = 1, 3 do
    local d = compare_numbers(x.core[i], y.core[i])
    if d ~= 0 then
      return d
    end
  end
  if #x.pre == 0 or #y.pre == 0 then
    return #x.pre == #y.pre and 0 or (#x.pre == 0 and 1 or -1)
  end
  for i = 1, math.min(#x.pre, #y.pre) do
    local d = compare_ids(x.pre[i], y.pre[i])
    if d ~= 0 then
      return d
    end
  end
  return #x.pre - #y.pre
end

local function order(tags)
  local list = {}
  for _, tag in ipairs(tags) do
    local v = parse(tag)
    if v then
      table.insert(list, v)
    else
      io.stderr:write("build.sh: " .. tag .. " is not a semantic version; left out\n")
    end
  end
  table.sort(list, function(x, y)
    local d = compare(x, y)
    if d ~= 0 then
      return d > 0
    end
    return x.tag > y.tag
  end)
  return list
end

-- latest returns the first tag that is not a prerelease, as go install @latest
-- does, or the first tag when all of them are prereleases. It returns nil when
-- there are no tags.
local function latest(tags)
  for _, tag in ipairs(tags) do
    if #parse(tag).pre == 0 then
      return tag
    end
  end
  return tags[1]
end

local function write_file(p, data)
  local f = assert(io.open(p, "wb"))
  f:write(data)
  f:close()
end

local function site(out, tags)
  local names = { "main" }
  for _, tag in ipairs(tags) do
    table.insert(names, tag)
  end
  write_file(out .. "/versions.json", pandoc.json.encode({ latest = latest(tags) or "main", main = os.getenv("DOCS_MAIN_SHA") or "", versions = names }) .. "\n")
  write_file(out .. "/index.html", [[
<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="color-scheme" content="light dark">
<title>Latest version</title>
<link rel="canonical" href="latest/">
<meta http-equiv="refresh" content="0; url=latest/">
</head>
<body>
<p><a href="latest/">Go to the latest version.</a></p>
</body>
</html>
]])
end

local command = table.remove(arg, 1)
if command == "order" then
  for _, v in ipairs(order(arg)) do
    print(v.tag)
  end
elseif command == "latest" then
  print(latest(arg) or "")
elseif command == "site" then
  site(table.remove(arg, 1), arg)
else
  io.stderr:write("build.sh: versions.lua has no command " .. tostring(command) .. "\n")
  os.exit(1)
end
