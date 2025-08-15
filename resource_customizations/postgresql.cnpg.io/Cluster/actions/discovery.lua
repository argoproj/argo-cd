local actions = {}
actions["restart"] = {
  ["iconClass"] = "fa fa-fw fa-plus",
  ["displayName"] = "Rollout restart Cluster"
}
actions["reload"] = {
  ["iconClass"] = "fa fa-fw fa-rotate-right",
  ["displayName"] = "Reload all Configuration"
}
actions["promote"] = {
  ["iconClass"] = "fa fa-fw fa-angles-up",
  ["displayName"] = "Promote Replica to Primary"
}
return actions
