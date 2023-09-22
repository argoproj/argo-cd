local hs = {}
if obj.status ~= nil then
    if obj.status.phase == "Pending" then
        hs.status = "Progressing"
        hs.message = "Experiment is pending"
    end
    if obj.status.phase == "Running" then
        hs.status = "Progressing"
        hs.message = "Experiment is running"
    end
    if obj.status.phase == "Successful" then
        hs.status = "Healthy"
        hs.message = "Experiment is successful"
    end
    if obj.status.phase == "Failed" then
        hs.status = "Degraded"
        hs.message = "Experiment has failed"
    end
    if obj.status.phase == "Error" then
        hs.status = "Degraded"
        hs.message = "Experiment had an error"
    end
    return hs
end

hs.status = "Progressing"
hs.message = "Waiting for experiment to finish: status has not been reconciled."
return hs