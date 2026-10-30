local hs = {}
if obj.status ~= nil and obj.status.status ~= nil then
  if obj.status.status == "Completed" then
    hs.status = "Healthy"
    hs.message = "ClickHouse installation completed successfully"
  elseif obj.status.status == "InProgress" then
    hs.status = "Progressing"
    hs.message = "ClickHouse installation in progress"
  else
    hs.status = "Degraded"
    hs.message = "ClickHouse status: " .. obj.status.status
  end
else
  hs.status = "Progressing"
  hs.message = "ClickHouse status not yet available"
end
return hs 