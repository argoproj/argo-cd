local os = require("os")

-- This action constructs a Workflow resource from a WorkflowTemplate resource, to enable creating a WorkflowTemplate instance
-- on demand.
-- It returns an array with a single member - a table with the operation to perform (create) and the Workflow resource.
-- It mimics the output of "argo submit --from=workflowtemplate/<WORKFLOW_TEMPLATE_NAME>" command, declaratively.

-- This code is written to mimic what the Argo Workflows API server does to create a Workflow from a WorkflowTemplate.
-- https://github.com/argoproj/argo-workflows/blob/873a58de7dd9dad76d5577b8c4294a58b52849b8/workflow/common/convert.go#L34

local workflow = {}
workflow.apiVersion = "argoproj.io/v1alpha1"
workflow.kind = "Workflow"

workflow.metadata = {}
workflow.metadata.name = obj.metadata.name .. "-" ..os.date("!%Y%m%d%H%M")
workflow.metadata.namespace = obj.metadata.namespace
workflow.metadata.labels = {}
workflow.metadata.labels["workflows.argoproj.io/workflow-template"] = obj.metadata.name

workflow.spec = {}
workflow.spec.workflowTemplateRef = {}
workflow.spec.workflowTemplateRef.name = obj.metadata.name

local ownerRef = {}
ownerRef.apiVersion = obj.apiVersion
ownerRef.kind = obj.kind
ownerRef.name = obj.metadata.name
ownerRef.uid = obj.metadata.uid
workflow.metadata.ownerReferences = {}
workflow.metadata.ownerReferences[1] = ownerRef

local impactedResource = {}
impactedResource.operation = "create"
impactedResource.resource = workflow
local result = {}
result[1] = impactedResource

return result
