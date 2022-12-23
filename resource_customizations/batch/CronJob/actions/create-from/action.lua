local os = require("os")

job = {}
job.apiVersion = "batch/v1"
job.kind = "Job"
job.metadata = {}
job.metadata.name = obj.spec.jobTemplate.metadata.name .. os.date("!%Y%m%d%H%M")
job.metadata.namespace = obj.metadata.namespace
job.metadata.ownerReferences = []
job.metadata.ownerReferences[0] = {}
job.metadata.ownerReferences[0].apiVersion = "batch/v1"
job.metadata.ownerReferences[0].kind = "CronJob"
job.metadata.ownerReferences[0].blockOwnerDeletion = "true"
job.metadata.ownerReferences[0].controller = "true"
job.metadata.ownerReferences[0].name = obj.spec.jobTemplate.metadata.name
job.metadata.ownerReferences[0].uid = obj.metadata.uid
job.spec = {}
job.spec.suspend = obj.spec.suspend
job.spec.template = {}
job.spec.template.spec = obj.spec.jobTemplate.spec
-- job.metadata.annotations["kubectl.kubernetes.io/createdAt"] = os.date("!%Y-%m-%dT%XZ")
file = io.open("job-created-from.yaml", "w")
file:write(job)
file:close()

obj.metadata.annotations["kuku"] = "muku"
return obj
