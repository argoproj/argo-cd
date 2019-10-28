hs = {}
if obj.status ~= nil then
    if obj.status.status == "Pending" then
        hs.status = "Progressing"
        hs.message = "Waiting for analysis run to finish: status is pending."
    end
    if obj.status.status == "Running" then
        hs.status = "Progressing"
        hs.message = "Waiting for analysis run to finish: status is running."
    end
    if obj.status.status == "Successful" then
        hs.status = "Healthy"
        hs.message = "Waiting for analysis run to finish: status is successful."
    end
    if obj.status.status == "Failed" then
        hs.status = "Degraded"
        hs.message = "Waiting for analysis run to finish: status is failed."
    end
    if obj.status.status == "Error" then
        hs.status = "Degraded"
        hs.message = "Waiting for analysis run to finish: status is error."
    end
    if obj.status.status == "Inconclusive" then
        hs.status = "Suspended"
        hs.message = "Waiting for analysis run to finish: status is inconclusive."
    end
    return hs
end

hs.status = "Progressing"
hs.message = "Waiting for analysis run to finish: status has not been reconciled."
return hs