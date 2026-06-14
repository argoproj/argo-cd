local hs = {}
if obj.status.status == "Succeeded" then
    hs.status = "Healthy"
    hs.message = "KeptnWorkloadVersion is healthy"
    return hs
end
if obj.status.status == "Failed" then
    hs.status = "Degraded"
    hs.message = "KeptnWorkloadVersion is degraded"
    return hs
end
hs.status = "Progressing"
hs.message = "KeptnWorkloadVersion is progressing"
return hs