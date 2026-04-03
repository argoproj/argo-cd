local hs = {}
if obj.status ~= nil then
  if obj.status.phase == "Bound" and obj.status.lastOperationStatus == "Success" then
    hs.status = "Healthy"
    hs.message = obj.status.message
    return hs
  end
  if obj.status.lastOperationStatus == "Failed" then
    hs.status = "Degraded"
    hs.message = obj.status.message
    return hs
  end
end
hs.status = "Progressing"
hs.message = "Waiting for backend creation"
return hs
