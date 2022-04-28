health_status = {}
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    msg = ""
    for i, condition in pairs(obj.status.conditions) do
      msg = msg .. i .. ": " .. condition.type .. " | " .. condition.status .. " "

      if condition.type == "Available" and condition.status == "True" then
        health_status.status = "Healthy"
      elseif condition.type == "Progressing" and condition.status == "True" then
        health_status.status = "Progressing"
      elseif (condition.type == "StorageSystemInvalid" and condition.status == "True") or (condition.type == "VendorCsvReady" and condition.status == "False") or (condition.type == "VendorSystemPresent" and condition.status == "False") then
        health_status.status = "Degraded"
      end

    end

    health_status.message = msg
    return health_status
  end
end
health_status.status = "Progressing"
health_status.message = "The StorageSystem is progressing"
return health_status
