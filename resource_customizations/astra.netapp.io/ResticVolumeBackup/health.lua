hs = { status = "Progressing", message = "No status available" }
if obj.status ~= nil then
  if obj.status.state ~= nil then
    if obj.status.state == "Completed" then
      hs.status = "Healthy"
      hs.message = obj.kind .. " Completed"
    elseif obj.status.state == "Running" then
      hs.status = "Progressing"
      hs.message = obj.kind .. " Running"
    else
      hs.status = "Degraded"
      hs.message = obj.status.state
    end
  end
end
return hs
