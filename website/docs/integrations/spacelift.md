---
title: Spacelift Integration
sidebar_position: 3
sidebar_label: Spacelift
---

Atmos natively supports [Spacelift](https://spacelift.io). This is accomplished using
the [`cloudposse/terraform-spacelift-cloud-infrastructure-automation`](https://github.com/cloudposse/terraform-spacelift-cloud-infrastructure-automation)
terraform module that reads the YAML Stack configurations and produces the Spacelift resources.

Cloud Posse provides two terraform components that implement Spacelift support.

- [Terraform Component](/core-concepts/components/) for provising
  a [Spacelift Worker Pool](https://github.com/cloudposse/terraform-aws-components/tree/master/modules/spacelift-worker-pool)
- [Terraform Component](/core-concepts/components/) for provisioning
  the [Spacelift Stacks](https://github.com/cloudposse/terraform-aws-components/tree/master/modules/spacelift)