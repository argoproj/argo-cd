local hs = {}

function messageOrDefault(field, default)
    if field ~= nil then
      return field
    end
    return default
  end

if obj.status ~= nil then
    if obj.status.phase == "Pending" then
        hs.status = "Progressing"
        hs.message = "Analysis run is running"
    end
    if obj.status.phase == "Running" then
        hs.status = "Progressing"
        hs.message = "Analysis run is running"
    end
    if obj.status.phase == "Successful" then
        hs.status = "Healthy"
        hs.message = messageOrDefault(obj.status.message, "Analysis run completed successfully")
    end
    if obj.status.phase == "Failed" then
        hs.status = "Degraded"
        hs.message = messageOrDefault(obj.status.message, "Analysis run failed")
    end
    if obj.status.phase == "Error" then
        hs.status = "Degraded"
        hs.message = messageOrDefault(obj.status.message, "Analysis run had an error")
    end
    if obj.status.phase == "Inconclusive" then
        hs.status = "Unknown"
        hs.message = messageOrDefault(obj.status.message, "Analysis run was inconclusive")
    end
    return hs
end

hs.status = "Progressing"
hs.message = "Waiting for analysis run to finish: status has not been reconciled."
return hs