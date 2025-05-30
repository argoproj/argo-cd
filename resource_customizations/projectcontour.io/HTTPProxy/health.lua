-- Status reporting information detailed here
-- https://projectcontour.io/docs/main/config/fundamentals/#status-reporting
hs = {
  status = "Progressing",
  message = "Waiting for status",
}

if obj.status == nil then
  hs.status = "Unknown"
  hs.message = "Cluster Status is unknown"
  if obj.status.currentStatus == "valid" then
    hs.status = "Healthy"
    hs.message = obj.status.description
  elseif obj.status.currentStatus == "invalid" then
    hs.status = "Degraded"
    hs.message = obj.status.description
  elseif obj.status.currentStatus == "error" then 
    hs.status = "Degraded"
    hs.message = "Error detected: " .. (obj.status.description or "No details")
  end
  
end

return hs