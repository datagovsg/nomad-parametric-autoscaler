![GitHub Logo](/images/nopas.png)
# NOmad-Parametric-AutoScaler
NOPAS is a template for a Go service that scales nomad tasks
* Parametric: policy is parameter-dependent -> can be changed dynamically via HTTP calls
* Auto: given a policy, its self-correcting
* Scaler: scales nomad tasks + EC2 instances

**Purpose**
Existing nomad metrics based autoscalers use CPU and Memory which is not sufficient for all use cases. At GovTech, our data scientists use Spark to crunch data on a daily basis. On one hand, it is costly to keep a large amount of compute resources ready at all times while on the other hand, using off-the-shelf cpu/memory-based autoscaling services may be too unresponsive.

NOPAS was built to enable users to easily add subpolicies based on more business-related needs such as pre-emptively scaling up resources in anticipation of user needs and scaling down outside of specific time periods to save cost. It comes with a simple UI for non-technical users.

## UI example

![UI](/images/ui.png)

## Running
Declare other env vars such as `ASG_ID` (AWS_ACCESS_KEY_ID), `ASG_SECRET` (AWS_SECRET_ACCESS_KEY), 
`VAULT_ADDR` and `VAULT_TOKEN` in an `.env` file.

```
export VAULT_TOKEN=$(cat ~/.vault-token)
docker-compose up
```

## API Endpoints

**Policy**
```
GET /policy
POST /policy
```

`POST` would require a policy in the form of an `application/json` as described below.

**State**
```
GET /state
PUT /state/pause
PUT /state/resume
```

`GET /state` expects a `200` and a `boolean` on whether NOPAS is running.

**Resouce Count**
```
GET /resource
```

Returns an object with keys representing resource and value representing current count of resource.
```json
{
    "Resource1":3,
    "Resource2":50
}
```

**Predefined**
```
GET /predefined
```

Returns an object with 2 fields each providing a list of predefined subpolicy and ensembler names.

```json
{
    "subpolicies": ["policy1", "policy2"],
    "ensemblers": ["conservative", "average"],
}
```

**Health**

GET `/ping` expects a status code of `200` and a `pong`

### Policy
A policy will govern how the scaling service manage the resources assigned to it. Each checking-scaliing cycle is performed by the policy in the following manner:

1. Each subpolicy will produce a map of resource to recommended count via various logic
2. Collate a list of recommended counts for each resource and resolve it via an ensembling method.
3. Each resource will check if the recommendation is within their allowable limits (`MaxCount` and `MinCount`) and if the previous scaling operation is within the cooldown period (`Cooldown`). 
4. If conditions are met, scaling is performed.

**Policy Structure**
1. Checking frequency
2. Resources
3. Subpolicies
4. Ensembling method

#### Checking Frequency
The autoscaling service will regularly initiate a checking-scaling cycle based on a user-defined time interval. The default checking frequency is 10s.

#### Resources
A resource refers to both the compute resource (e.g. EC2) and the nomad client. Resources are independent of other resources in the eyes of the policy when scaling is performed.

Resource Definition
* EC2 (see below)
* Nomad (see below)
* Cooldown - minimum duration between scaling. A required string indicating duration, e.g. "1m40s".
* N2CRatio - ratio of nomad to compute resources. A required number.


*EC2 Definition*

| Name | Description | Type  | Required |
|------|-------------|:----:|:-----:|
| ScalingGroupName | AWS EC2 auto-scaling group name | string | Yes |
| Region | AWS service region, e.g. `ap-southeast-1` | string | Yes |
| MaxCount | Maximum allowable desired count | number | Yes |
| MinCount | Minimum allowable desired count | number | Yes |

*Nomad Definition*

| Name | Description | Type  | Required |
|------|-------------|:----:|:-----:|
| Address | Address of nomad service | string | Yes |
| JobName | Name of nomad job to be tracking and updating | string | Yes |
| NomadPath | Vault path for Nomad ACL token | string | Yes |
| MaxCount | Maximum allowable desired count | number | Yes |
| MinCount | Minimum allowable desired count | number | Yes |


```json
"Example": {
            "EC2": {
                "ScalingGroupName": "<<auto scaling group name>>",
                "Region": "ap-southeast-1",
                "MaxCount": 25,
                "MinCount": 1
            },
            "Nomad": {
                "Address": "<nomad address>",
                "JobName": "<nomad job name>",
                "NomadPath": "<secret's path>",
                "MaxCount": 25,
                "MinCount": 1
            },
            "Cooldown": "1m0s",
            "N2CRatio": 1
        }
```

#### Subpolicy
Subpolicies outline the logic behind deriving a recommended nomad task-group count. 

Each sub-policy will 
1. track a metric
2. recommend counts for resources under its management
 
 **Implementing a new subpolicy**
Users can implement their own custom subpolicy by implementing the `Subpolicy` interface and follow the `GenericSubPolicy` structure. Two examples, `core_ratio_subpolicy` and `office_hour_subpolicy` have been implemented.

Core ratio subpolicy tracks a Spark master endpoint to find out the core usage and scales accordingly while the office hour subpolicy keeps a minimum count of resources between predefined hours.

**Subpolicy API**

| Name | Description | Type  | Required |
|------|-------------|:----:|:-----:|
| Name | Name of subpolicy. *Important* This name needs to match the string in the `CreateSpecificSubpolicy` function. | string | Yes |
| ManagedResources | List of resource to be managed by subpolicy. Resource name needs to match corresponding resource key in `Resources` part of the policy definition | array[string] | Yes |
| Metadata | Metadata specific to sub-policy. | Object | Yes |
For example:
```json
{
    "Name": "CoreRatio",
    "ManagedResources": [
        "SparkWorker"
    ],
    "Metadata": {
        "MetricSource": "https://some-endpoint",
        "UpThreshold": 0.5,
        "DownThreshold": 0.25,
        "ScaleUp": {
            "Changetype": "multiply",
            "ChangeValue": 2
        },
        "ScaleDown": {
            "Changetype": "multiply",
            "ChangeValue": 0.5
        }
    }
}
```

#### Ensembling
Given that each subpolicy will recommend a count, `Ensemble.go` provides ensembling methods to resolve multiple recommendations. 

Users can implement their own methods by implementing the `Ensembler` interface.

Various ensembling methods can be considered for each resource
1. Conservative(takes the maximum to be safe)
2. Averaging
3. Cost-saving(takes the minimum to save cost)


### Example Policy JSON Definition

```json
{
    "CheckingFreq": "10s",
    "Resources": {
        "ImportantJob": {
            "EC2": {
                "ScalingGroupName": "group_name",
                "Region": "ap-southeast-1",
                "MaxCount": 25,
                "MinCount": 1
            },
            "Nomad": {
                "Address": "https://example.nomad.address",
                "JobName": "important_job",
                "NomadPath": "",
                "MaxCount": 25,
                "MinCount": 1
            },
            "Cooldown": "1m0s",
            "N2CRatio": 1
        }
    },
    "Subpolicies": [
        {
        "Name": "CoreRatio",
        "ManagedResources": [
            "SparkWorker"
        ],
        "Metadata": {
            "MetricSource": "https://some-endpoint",
            "UpThreshold": 0.5,
            "DownThreshold": 0.25,
            "ScaleOut": {
                "Changetype": "multiply",
                "ChangeValue": 2
            },
            "ScaleIn": {
                "Changetype": "multiply",
                "ChangeValue": 0.5
            }
        }
    }
    ],
    "Ensembler": "Conservative"
}
```
