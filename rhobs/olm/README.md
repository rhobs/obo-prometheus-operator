# Explanation of olm dir


```
├── bundle    # bundle and its subdir is generated from make bundle
├── catalog   # contains all files to build olm catalog
│   └── config   config contains the catalog index.yaml file
│
├── manifests    # manifests is the source for building bundle
│   │            # it is split into subdirs which contains each of the PO's 
│   │            # components - admission-webhook, operator
│   │
│   ├── admission-webhook
│   │   ├── additional
│   │   │
│   │   │  # example directory is a copy of the example from PO, that is used 
│   │   │  # to build the component - admission-webhook in this case
│   │   └── example
│   │       └── admission-webhook
│   │
│   ├── crds
│   │   └── example
│   │       └── stripped-down-crds
│   │
│   ├── csv  # contains the base rhobs-prometheus-operator CSV 
│   │
│   ├── operator  # the operator component
│   │   └── example
│   │       └── rbac
│   │           ├── prometheus
│   │           ├── prometheus-operator
│   │           └── prometheus-operator-crd
│   └── scorecard
│       ├── bases
│       └── patches
│
└── subscription # contains olm subscription and catalog-source yaml
```

