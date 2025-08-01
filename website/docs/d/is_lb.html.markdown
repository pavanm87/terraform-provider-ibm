---
subcategory: "VPC infrastructure"
layout: "ibm"
page_title: "IBM : load balancer"
description: |-
  Manages IBM Cloud VPC load balancer.
---

# ibm_is_lb
Retrieve information of an existing IBM VPC Load Balancer. For more information, about VPC load balancer, see [load balancers for VPC overview](https://cloud.ibm.com/docs/vpc?topic=vpc-nlb-vs-elb).

**Note:** 
VPC infrastructure services are a regional specific based endpoint, by default targets to `us-south`. Please make sure to target right region in the provider block as shown in the `provider.tf` file, if VPC service is created in region other than `us-south`.

**provider.tf**

```terraform
provider "ibm" {
  region = "eu-gb"
}
```

## Example usage

```terraform
resource "ibm_is_vpc" "example" {
  name = "example-vpc"
}

resource "ibm_is_subnet" "example" {
  name            = "example-subnet"
  vpc             = ibm_is_vpc.example.id
  zone            = "us-south-1"
  ipv4_cidr_block = "10.240.0.0/24"
}

resource "ibm_is_lb" "example" {
  name    = "example-lb"
  subnets = [ibm_is_subnet.example.id]
}

data "ibm_is_lb" "example" {
  name = ibm_is_lb.example.name
}
```

## Argument reference
Review the argument references that you can specify for your data source. 
 
- `name` - (Required, String) The name of the load balancer. 

## Attribute reference
In addition to all argument reference list, you can access the following attribute references after your data source is created. 

- `access_mode` - (String) The access mode for this load balancer. One of **private**, **public**, **private_path**.
- `access_tags`  - (String) Access management tags associated for the load balancer.
- `attached_load_balancer_pool_members` - (List) The load balancer pool members attached to this load balancer.
	Nested scheme for `members`:
	- `deleted` - (List) If present, this property indicates the referenced resource has been deleted and providessome supplementary information.
		Nested scheme for `deleted`:
    	- `more_info` - (String) Link to documentation about deleted resources.
    - `href` - (String) The URL for this load balancer pool member.
    - `id` - (String) The unique identifier for this load balancer pool member.
- `availability` - (String) The availability of this load balancer
- `crn` - (String) The CRN for this load balancer.
- `dns` - (List) The DNS configuration for this load balancer.

  Nested scheme for `dns`:
  - `instance_crn` - (String) The CRN of the DNS instance associated with the DNS zone
  - `zone_id` - (String) The unique identifier of the DNS zone.
- `failsafe_policy_actions` - (List) The supported `failsafe_policy.action` values for this load balancer's pools. Allowable list items are: [ `bypass`, `drop`, `fail`, `forward` ]. 
    A load balancer failsafe policy action:
    - `bypass`: Bypasses the members and sends requests directly to their destination IPs.
    - `drop`: Drops requests.
    - `fail`: Fails requests with an HTTP 503 status code.
    - `forward`: Forwards requests to the target pool.
- `hostname` - (String) Fully qualified domain name assigned to this load balancer.
- `id` - (String) The ID of the load balancer.
- `instance_groups_supported` - (Boolean) Indicates whether this load balancer supports instance groups.
- `listeners` - (String) The ID of the listeners attached to this load balancer.
- `logging`-  (Bool) Enable (**true**) or disable (**false**) datapath logging for this load balancer. If unspecified, datapath logging is disabled. This option is supported only for application load balancers.
- `operating_status` - (String) The operating status of this load balancer.
- `pools` - (List) List all the Pools attached to this load balancer.

  Nested scheme for `pools`:
	- `algorithm` - (String) The load balancing algorithm.
	- `created_at` -  (String) The date and time pool was created.
	- `href` - (String) The pool's canonical URL.
	- `id` - (String) The unique identifier for this load balancer pool.
	- `name` - (String) The user-defined name for this load balancer pool.
	- `protocol` - (String) The protocol used for this load balancer pool.
	- `provisioning_status` - (String) The provisioning status of this pool.
	- `health_monitor` - (List) The health monitor of this pool.

	  Nested scheme for `health_monitor`:
	  - `delay` - (String) The health check interval in seconds. Interval must be greater than timeout value.
	  - `max_retries` - (String) The health check max retries.
	  - `timeout` - (String) The health check timeout in seconds.
	  - `type` - (String) The protocol type of this load balancer pool health monitor.
	  - `url_path` - (String) The health monitor of this pool.
  - `instance_group` - (List) The instance group that is managing this pool.

    Nested scheme for `instance_group`:
	- `crn` - (String) The CRN for this instance group.
	- `href` - (String) The URL for this instance group.
	- `id` - (String) The unique identifier for this instance group.
	- `name` - (String) The user-defined name for this instance group.
  - `members` - (List) The backend server members of the pool.

    Nested scheme for `members`:
	- `href` - (String) The canonical URL of the member.
	- `id` - (String) The unique identifier for this load balancer pool member.
  - `session_persistence` - (List) The session persistence of this pool.

    Nested scheme for `session_persistence`:
	- `type` - (String) The session persistence type.
- `public_ips` - (String) The public IP addresses assigned to this load balancer.
- `private_ip` - (List) The primary IP address to bind to the network interface. This can be specified using an existing reserved IP, or a prototype object for a new reserved IP.

	Nested scheme for `private_ip`:
	- `address` - (String) The IP address. If the address has not yet been selected, the value will be 0.0.0.0. This property may add support for IPv6 addresses in the future. When processing a value in this property, verify that the address is in an expected format. If it is not, log an error. Optionally halt processing and surface the error, or bypass the resource on which the unexpected IP address format was encountered.
	- `href`- (String) The URL for this reserved IP
	- `name`- (String) The user-defined or system-provided name for this reserved IP
	- `reserved_ip`- (String) The unique identifier for this reserved IP
	- `resource_type`- (String) The resource type.
- `private_ips` - (List) The private IP addresses assigned to this load balancer. Same as `private_ip.[].address`
- `resource_group` - (String) The resource group id, where the load balancer is created.
- `route_mode` - (Bool) Indicates whether route mode is enabled for this load balancer.
- `security_groups`- (String) A list of security groups that are used with this load balancer. This option is supported only for application load balancers.
- `security_groups_supported`- (Bool) Indicates if this load balancer supports security groups.
- `source_ip_session_persistence_supported` - (Boolean) Indicates whether this load balancer supports source IP session persistence.
- `subnets` - (String) The ID of the subnets to provision this load balancer.
- `status` - (String) The status of load balancer.
- `tags` - (String) The tags associated with the load balancer.
- `type` - (String) The type of the load balancer.
- `udp_supported`- (Bool) Indicates whether this load balancer supports UDP.
