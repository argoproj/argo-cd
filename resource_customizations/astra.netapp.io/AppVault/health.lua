hs = { status = "Progressing", message = "No status available" }
if obj.status ~= nil then
  if obj.status.state ~= nil then
    if obj.status.state == "available" or obj.status.state == "Available" then
      hs.status = "Healthy"
      hs.message = obj.kind .. " Available"
    elseif obj.status.state == "failed" or obj.status.state == "Failed" then
      hs.status = "Degraded"
      hs.message = obj.kind .. " Failed"
    end
  end
end
return hs
