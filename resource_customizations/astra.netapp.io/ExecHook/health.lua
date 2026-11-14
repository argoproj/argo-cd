hs = { status = "Progressing", message = "No status available" }
if obj.spec ~= nil then
  if obj.spec.enabled ~= nil then
    if obj.spec.enabled == true then
      hs.status = "Healthy"
      hs.message = obj.kind .. " enabled"
    elseif obj.spec.enabled == false then
      hs.status = "Suspended"
      hs.message = obj.kind .. " disabled"
    end
  end
end
return hs
