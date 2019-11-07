hs = {}
if obj.status ~= nil then
    if obj.status.status == "Pending" then
        hs.status = "Progressing"
        hs.message = "Analysis run is running"
    end
    if obj.status.status == "Running" then
        hs.status = "Progressing"
        hs.message = "Analysis run is running"
    end
    if obj.status.status == "Successful" then
        hs.status = "Healthy"
        hs.message = "Analysis run completed successfully"
    end
    if obj.status.status == "Failed" then
        hs.status = "Degraded"
        hs.message = "Analysis run failed"
    end
    if obj.status.status == "Error" then
        hs.status = "Degraded"
        hs.message = "Analysis run had an error"
    end
    if obj.status.status == "Inconclusive" then
        hs.status = "Unknown"
        hs.message = "Analysis run was inconclusive"
    end
    return hs
end

hs.status = "Progressing"
hs.message = "Waiting for analysis run to finish: status has not been reconciled."
return hs