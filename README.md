# InvokeX
Welcome to InvokeX - a simple implementation of serverless infrastructure.

## Project scope
This project is meant to be an educational project to learn how AWS's serverless infrastructure works.
InvokeX is built upon the [same microvm technology](https://github.com/firecracker-microvm/firecracker) as [AWS Lambda](https://docs.aws.amazon.com/lambda/).

While you are more than welcome to try and run production workloads on InvokeX, there are absolutely no guarantees whatsoever regarding stability, support, and reliability.

## Architecture
At its core, InvokeX is a harness over Firecracker.
However, InvokeX is a bit more than just a hypervisor for Firecracker VMs.
InvokeX is a modular system that allows developers to upload their applications to InvokeX and it will magically run as a serverless workload.

```mermaid
architecture-beta
    group control-layer(server)[control plane]

    group firecracker0(server)[firecracker 0] in control-layer
    group firecracker1(server)[firecracker 1] in control-layer
    group firecracker2(server)[firecracker 2] in control-layer
    group firecrackerN(server)[firecracker N] in control-layer

    junction control-junction-t in control-layer
    junction control-junction-c in control-layer
    junction control-junction-b in control-layer
    
    service fe(internet)[frontend]
    fe:B -- T:control

    service control(server)[control]
    service control-code-db(database)[applications]
    service control-runtime-db(database)[runtimes]
    control:L -- R:control-code-db
    control:R -- L:control-runtime-db
    control:B -- T:control-junction-t
    control-junction-t:B -- T:control-junction-c
    control-junction-c:B -- T:control-junction-b
    
    service harness0(server)[harness] in firecracker0
    harness0:R -- L:control-junction-t
    service vm0(disk)[microvm] in firecracker0
    harness0:B -- T:vm0
    
    service harness1(server)[harness] in firecracker1
    harness1:R -- L:control-junction-b
    service vm1(disk)[microvm] in firecracker1
    harness1:B -- T:vm1
    
    service harness2(server)[harness] in firecracker2
    harness2:L -- R:control-junction-t
    service vm2(disk)[microvm] in firecracker2
    harness2:B -- T:vm2
    
    service harnessN(server)[harness] in firecrackerN
    harnessN:L -- R:control-junction-b
    service vmN(disk)[microvm] in firecrackerN
    harnessN:B -- T:vmN
```
