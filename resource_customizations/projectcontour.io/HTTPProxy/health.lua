-- Status reporting information detailed here
-- https://projectcontour.io/docs/main/config/fundamentals/#status-reporting
hs = {
  status = "Progressing",
  message = "Waiting for status",
}

if obj.status == nil then
  hs.status = "Unknown"
  hs.message = "Cluster Status is unknown"
elseif obj.status.currentStatus == "valid" then
  hs.status = "Healthy"
  hs.message = obj.status.description
elseif obj.status.currentStatus == "invalid" then
  hs.status = "Degraded"
  hs.message = obj.status.description
elseif obj.status.currentStatus == "orphaned" or obj.status.currentStatus == "NotReconciled" then 
  hs.status = "Degraded"
  hs.message = "Error detected: " .. (obj.status.description or "No details")
end

return hs