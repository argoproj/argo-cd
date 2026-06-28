local hs = {}
if obj.status.overallStatus == "Succeeded" then
    hs.status = "Healthy"
    hs.message = "KeptnEvaluation is healthy"
    return hs
end
if obj.status.overallStatus == "Failed" then
    hs.status = "Degraded"
    hs.message = "KeptnEvaluation is degraded"
    return hs
end
hs.status = "Progressing"
hs.message = "KeptnEvaluation is progressing"
return hs