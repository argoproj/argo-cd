local k8s = import "k8s.libsonnet";

local apps = k8s.apps;
local core = k8s.core;
local extensions = k8s.extensions;

local hidden = {
  mapContainers(f):: {
    local podContainers = super.spec.template.spec.containers,
    spec+: {
      template+: {
        spec+: {
          // IMPORTANT: This overwrites the 'containers' field
          // for this deployment.
          containers: std.map(f, podContainers),
        },
      },
    },
  },

  mapContainersWithName(names, f) ::
    local nameSet =
      if std.type(names) == "array"
      then std.set(names)
      else std.set([names]);
    local inNameSet(name) = std.length(std.setInter(nameSet, std.set([name]))) > 0;
    self.mapContainers(
      function(c)
        if std.objectHas(c, "name") && inNameSet(c.name)
        then f(c)
        else c
    ),
};

k8s + {
  apps:: apps + {
    v1beta1:: apps.v1beta1 + {
      local v1beta1 = apps.v1beta1,

      daemonSet:: v1beta1.daemonSet + {
        mapContainers(f):: hidden.mapContainers(f),
        mapContainersWithName(names, f):: hidden.mapContainersWithName(names, f),
      },

      deployment:: v1beta1.deployment + {
        mapContainers(f):: hidden.mapContainers(f),
        mapContainersWithName(names, f):: hidden.mapContainersWithName(names, f),
      },
    },
  },

  core:: core + {
    v1:: core.v1 + {
      list:: {
        new(items)::
          {apiVersion: "v1"} +
          {kind: "List"} +
          self.items(items),

        items(items):: if std.type(items) == "array" then {items+: items} else {items+: [items]},
      },
    },
  },

  extensions:: extensions + {
    v1beta1:: extensions.v1beta1 + {
      local v1beta1 = extensions.v1beta1,

      daemonSet:: v1beta1.daemonSet + {
        mapContainers(f):: hidden.mapContainers(f),
        mapContainersWithName(names, f):: hidden.mapContainersWithName(names, f),
      },

      deployment:: v1beta1.deployment + {
        mapContainers(f):: hidden.mapContainers(f),
        mapContainersWithName(names, f):: hidden.mapContainersWithName(names, f),
      },
    },
  },
}
