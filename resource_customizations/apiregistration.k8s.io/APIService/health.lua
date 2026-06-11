local hs = {}

if obj.status ~= nil and obj.status.conditions ~= nil then
  for _, c in ipairs(obj.status.conditions) do
    if c.type == "Available" then
      local msg = string.format("%s: %s", c.reason or "", c.message or "")
      if c.status == "True" then
        hs.status = "Healthy"
        hs.message = msg
        return hs
      end
      hs.status = "Progressing"
      hs.message = msg
      return hs
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting to be processed"
return hs
