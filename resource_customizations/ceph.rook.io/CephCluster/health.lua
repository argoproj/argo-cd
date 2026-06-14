-- CRD documentation: https://rook.github.io/docs/rook/latest-release/CRDs/Cluster/ceph-cluster-crd/
local hs = {
    status = "Progressing",
    message = ""
}

function append_to_message(message)
    if message ~= "" then
        if hs.message ~= "" then
            hs.message = hs.message .. " - " .. message
        else
            hs.message = message
        end
    end
end

if obj.status == nil then
    append_to_message("Waiting for status to be reported")
    return hs
end

-- Check the main Ceph health status first - https://github.com/ceph/ceph/blob/v20.3.0/src/include/health.h#L12
if obj.status.ceph ~= nil and obj.status.ceph.health ~= nil then
    local ceph_health = obj.status.ceph.health
    local details_message = ""

    -- Build details message from status.ceph.details if available
    if obj.status.ceph.details ~= nil then
        local detail_parts = {}
        local sorted_detail_types = {}
        for detail_type, _ in pairs(obj.status.ceph.details) do
            table.insert(sorted_detail_types, detail_type)
        end
        table.sort(sorted_detail_types)
        for _, detail_type in ipairs(sorted_detail_types) do
            local detail_info = obj.status.ceph.details[detail_type]
            if detail_info.message ~= nil then
                table.insert(detail_parts, detail_info.message)
            end
        end
        if #detail_parts > 0 then
            details_message =  " (" .. table.concat(detail_parts, "; ") .. ")"
        end
    end

    if ceph_health == "HEALTH_ERR" or ceph_health == "HEALTH_WARN" then
        hs.status = "Degraded"
    elseif ceph_health == "HEALTH_OK" then
        hs.status = "Healthy"
    end
    append_to_message("Ceph health is " .. ceph_health .. details_message)
end

-- Check state - https://github.com/rook/rook/blob/v1.17.7/pkg/apis/ceph.rook.io/v1/types.go#L621
if obj.status.state ~= nil then
    if hs.status == "Healthy" then
        append_to_message("Ceph cluster state is " .. obj.status.state)
        if obj.status.state == "Created" or obj.status.state == "Connected" then
            hs.status = "Healthy"
        elseif obj.status.state == "Error" then
            hs.status = "Degraded"
        else
            hs.status = "Progressing"
        end
    end
end

if obj.status.message ~= nil then
    append_to_message(obj.status.message)
end

return hs
