hs = {}
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
        hs.message = "Analysis run completed successfully"
    end
    if obj.status.phase == "Failed" then
        hs.status = "Degraded"
        if obj.status.message ~= nil then
            hs.message = obj.status.message
        else
            hs.message = "Analysis run failed"
        end
    end
    if obj.status.phase == "Error" then
        hs.status = "Degraded"
        if obj.status.message ~= nil then
            hs.message = obj.status.message
        else
            hs.message = "Analysis run had an error"
        end
    end
    if obj.status.phase == "Inconclusive" then
        hs.status = "Unknown"
        if obj.status.message ~= nil then
            hs.message = obj.status.message
        else
            hs.message = "Analysis run was inconclusive"
        end
    end
    return hs
end

hs.status = "Progressing"
hs.message = "Waiting for analysis run to finish: status has not been reconciled."
return hs