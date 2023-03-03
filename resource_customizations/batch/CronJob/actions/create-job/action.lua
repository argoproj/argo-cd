local os = require("os")
job = {}
job.apiVersion = "batch/v1"
job.kind = "Job"
job.metadata = {}
job.metadata.name = obj.metadata.name .. os.date("!%Y%m%d%H%M")
job.metadata.namespace = obj.metadata.namespace
job.spec = {}
job.spec.template = {}
job.spec.template.spec = obj.spec.jobTemplate.spec.template.spec
job.metadata.ownerReferences = {}
ownerRef = {}
ownerRef.apiVersion = obj.apiVersion
ownerRef.kind = "CronJob"
ownerRef.name = obj.metadata.name
ownerRef.uid = obj.metadata.uid
-- job.metadata.ownerReferences[0] = ownerRef
return job