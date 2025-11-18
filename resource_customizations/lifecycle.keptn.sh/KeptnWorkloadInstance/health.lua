local hs = {}
if obj.status.status == "Succeeded" then
    hs.status = "Healthy"
    hs.message = "KeptnWorkloadInstance is healthy"
    return hs
end
if obj.status.status == "Failed" then
    hs.status = "Degraded"
    hs.message = "KeptnWorkloadInstance is degraded"
    return hs
end
hs.status = "Progressing"
hs.message = "KeptnWorkloadInstance is progressing"
return hs