local os = require("os")

-- This action constructs a Job resource from a CronJob resource, to enable creating a CronJob instance on demand.
-- It returns an array with a single member - a table with the operation to perform (create) and the Job resource.
-- It mimics the output of "kubectl create job --from=<CRON_JOB_NAME>" command, declaratively.

-- Deep-copying an object is a ChatGPT generated code.
-- Since empty tables are treated as empty arrays, the resulting k8s resource might be invalid (arrays instead of maps).
-- So empty tables are not cloned to the target object.
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

local job = {}
job.apiVersion = "batch/v1"
job.kind = "Job"

job.metadata = deepCopy(obj.spec.jobTemplate.metadata)
if job.metadata == nil then
  job.metadata = {}
end
job.metadata.name = obj.metadata.name .. "-" ..os.date("!%y%m%d%H%M")
job.metadata.namespace = obj.metadata.namespace
if job.metadata.annotations == nil then
  job.metadata.annotations = {}
end
job.metadata.annotations['cronjob.kubernetes.io/instantiate'] = "manual"

local ownerRef = {}
ownerRef.apiVersion = obj.apiVersion
ownerRef.kind = obj.kind
ownerRef.name = obj.metadata.name
ownerRef.uid = obj.metadata.uid
ownerRef.blockOwnerDeletion = true
ownerRef.controller = true
job.metadata.ownerReferences = {}
job.metadata.ownerReferences[1] = ownerRef

job.spec = deepCopy(obj.spec.jobTemplate.spec)

local impactedResource = {}
impactedResource.operation = "create"
impactedResource.resource = job
local result = {}
result[1] = impactedResource

return result
