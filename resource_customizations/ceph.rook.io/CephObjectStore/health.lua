-- CRD documentation: https://rook.github.io/docs/rook/latest-release/CRDs/Object-Storage/ceph-object-store-crd/
-- Status documentation: https://github.com/rook/rook/blob/v1.17.7/pkg/apis/ceph.rook.io/v1/types.go#L1960
local hs = {
    status = "Progressing",
    message = "Waiting for status to be reported"
}

if obj.status == nil then
    return hs
end

-- phase status check - https://github.com/rook/rook/blob/v1.17.7/pkg/apis/ceph.rook.io/v1/types.go#L596
if obj.status.phase ~= nil then
    hs.message = "Ceph object store phase is " .. obj.status.phase
    if obj.status.phase == "Ready" then
        hs.status = "Healthy"
    elseif obj.status.phase == "Failure" then
        hs.status = "Degraded"
    end
end

if obj.status.info ~= nil and obj.status.info.endpoint ~= nil and obj.status.info.endpoint ~= "" then
    hs.message = hs.message .. " - endpoint: " .. obj.status.info.endpoint
end

return hs
