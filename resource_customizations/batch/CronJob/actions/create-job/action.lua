local os = require("os")

function deepCopy(obj, seen)
    -- Handle non-tables and previously-seen tables.
    if type(obj) ~= 'table' then 
        return obj end
    if seen and seen[obj] then return seen[obj] end
  
    -- New table; mark it as seen and copy recursively.
    local s = seen or {}
    local res = {}
    s[obj] = res
    for k, v in pairs(obj) do res[deepCopy(k, s)] = deepCopy(v, s) end
    return setmetatable(res, getmetatable(obj))
end

job = {}
job.apiVersion = "batch/v1"
job.kind = "Job"

job.metadata = {}
job.metadata.name = obj.metadata.name .. "-" ..os.date("!%Y%m%d%H%M")
job.metadata.namespace = obj.metadata.namespace
job.metadata.ownerReferences = {}

ownerRef = {}
ownerRef.apiVersion = obj.apiVersion
ownerRef.kind = obj.kind
ownerRef.name = obj.metadata.name
ownerRef.uid = obj.metadata.uid

job.metadata.ownerReferences[1] = ownerRef

job.spec = {}
job.spec.suspend = false
job.spec.template = {}

job.spec.template.spec = deepCopy(obj.spec.jobTemplate.spec.template.spec)
print ("deep copied")
print (job.spec.template.spec)
return job

