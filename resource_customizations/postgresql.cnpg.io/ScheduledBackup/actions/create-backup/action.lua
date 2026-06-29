local os = require("os")

-- This action constructs a postgresql.cnpg.io/v1/Backup resource
-- from a postgresql.cnpg.io/v1/ScheduledBackup resource.
-- Heavily inspired on the CronJob create-job action.

-- deepCopy function copied from CronJob create-job action
function deepCopy(object)
    local lookup_table = {}
    local function _copy(obj)
        if type(obj) ~= "table" then
            return obj
        elseif lookup_table[obj] then
            return lookup_table[obj]
        elseif next(obj) == nil then
            return nil
        else
            local new_table = {}
            lookup_table[obj] = new_table
            for key, value in pairs(obj) do
                new_table[_copy(key)] = _copy(value)
            end
            return setmetatable(new_table, getmetatable(obj))
        end
    end
    return _copy(object)
end

local backup = {}

backup.apiVersion = "postgresql.cnpg.io/v1"
backup.kind = "Backup"

backup.metadata = {}
backup.metadata.name = obj.metadata.name .. "-" .. os.date("!%Y%m%d%H%M%S")
backup.metadata.namespace = obj.metadata.namespace
backup.metadata.labels = {}
backup.metadata.labels["cnpg.io/cluster"] = obj.spec.cluster.name
backup.metadata.labels["cnpg.io/scheduled-backup"] = obj.metadata.name

local ownerRef = {}
ownerRef.apiVersion = obj.apiVersion
ownerRef.kind = obj.kind
ownerRef.name = obj.metadata.name
ownerRef.uid = obj.metadata.uid
ownerRef.blockOwnerDeletion = true
ownerRef.controller = true
backup.metadata.ownerReferences = {}
backup.metadata.ownerReferences[1] = ownerRef

backup.spec = {}

backup.spec.cluster = deepCopy(obj.spec.cluster)

if obj.spec.target ~= nil then
  backup.spec.target = obj.spec.target
end

if obj.spec.method ~= nil then
  backup.spec.method = obj.spec.method
end

if obj.spec.pluginConfiguration ~= nil then
  backup.spec.pluginConfiguration = deepCopy(obj.spec.pluginConfiguration)
end

if obj.spec.online ~= nil then
  backup.spec.online = obj.spec.online
end

if obj.spec.onlineConfiguration ~= nil then
  backup.spec.onlineConfiguration = deepCopy(obj.spec.onlineConfiguration)
end

local impactedResource = {}
impactedResource.operation = "create"
impactedResource.resource = backup
local result = {}
result[1] = impactedResource

return result
