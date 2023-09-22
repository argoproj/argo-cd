local hs = {}
if obj.status.status == "Succeeded" then
    hs.status = "Healthy"
    hs.message = "KeptnAppVersion is healthy"
    return hs
end
if obj.status.status == "Failed" then
    hs.status = "Degraded"
    hs.message = "KeptnAppVersion is degraded"
    return hs
end
hs.status = "Progressing"
hs.message = "KeptnAppVersion is progressing"
return hs