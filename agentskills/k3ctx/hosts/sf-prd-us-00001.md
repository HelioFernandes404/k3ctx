---
systemframe_id: sf-prd-us-00001
context_name: systemframe-sf-prd-us-00001
netbird_fqdn: sf-prd-us-00001.systemframe.vpn
aliases:
  - prod-primaria
  - primary-prod
  - prd-us-1
  - prod-us-1
description: Primary production cluster (EC2, US)
tags:
  - production
  - ec2
  - k3s
default_namespaces:
  - kube-system
  - argocd
  - monitoring
  - default
---
