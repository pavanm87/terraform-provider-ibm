// Copyright IBM Corp. 2017, 2021 All Rights Reserved.
// Licensed under the Mozilla Public License v2.0

package hpcs

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"reflect"
	"strconv"

	"github.com/IBM/ibm-hpcs-tke-sdk/tkesdk"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/IBM-Cloud/terraform-provider-ibm/ibm/conns"
	"github.com/IBM-Cloud/terraform-provider-ibm/ibm/flex"
	rc "github.com/IBM/platform-services-go-sdk/resourcecontrollerv2"
)

func DataSourceIBMHPCS() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceIBMHPCSRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Description: "Resource instance name for example, myobjectstorage",
				Type:        schema.TypeString,
				Required:    true,
			},

			"resource_group_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "The id of the resource group in which the instance is present",
			},

			"location": {
				Description: "The location or the environment in which instance exists",
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
			},

			"service": {
				Description: "The service type of the instance",
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "hs-crypto",
			},
			"units": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The number of operational crypto units for your service instance",
			},
			"failover_units": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The number of failover crypto units for your service instance",
			},
			"service_endpoints": {
				Description: "Types of the service endpoints. Possible values are `public-and-private`, `private-only`.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"plan": {
				Description: "The plan type of the instance",
				Type:        schema.TypeString,
				Computed:    true,
			},

			"status": {
				Description: "The resource instance status",
				Type:        schema.TypeString,
				Computed:    true,
			},

			"crn": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "CRN of resource instance",
			},

			"guid": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Guid of resource instance",
			},
			"extensions": {
				Type:        schema.TypeMap,
				Computed:    true,
				Description: "The extended metadata as a map associated with the resource instance.",
			},
			"hsm_info": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "HSM Info of HPCS CryptoUnits",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"hsm_id": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"hsm_location": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"hsm_type": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"signature_threshold": {
							Type:     schema.TypeInt,
							Computed: true,
						},
						"revocation_threshold": {
							Type:     schema.TypeInt,
							Computed: true,
						},
						"current_mk_status": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"new_mk_status": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"current_mkvp": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"new_mkvp": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"admins": {
							Type:     schema.TypeList,
							Computed: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"name": {
										Type:     schema.TypeString,
										Computed: true,
									},
									"ski": {
										Type:     schema.TypeString,
										Computed: true,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func dataSourceIBMHPCSRead(context context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	rsConClient, err := meta.(conns.ClientSession).ResourceControllerV2API()
	if err != nil {
		return diag.FromErr(err)
	}
	name := d.Get("name").(string)

	resourceInstanceListOptions := rc.ListResourceInstancesOptions{
		Name: &name,
	}

	if rsGrpID, ok := d.GetOk("resource_group_id"); ok {
		rg := rsGrpID.(string)
		resourceInstanceListOptions.ResourceGroupID = &rg
	}

	rsCatClient, err := meta.(conns.ClientSession).ResourceCatalogAPI()
	if err != nil {
		return diag.FromErr(err)
	}
	rsCatRepo := rsCatClient.ResourceCatalog()

	if service, ok := d.GetOk("service"); ok {

		serviceOff, err := rsCatRepo.FindByName(service.(string), true)
		if err != nil {
			return diag.FromErr(fmt.Errorf("[ERROR] Error retrieving service offering: %s", err))
		}
		resourceId := serviceOff[0].ID
		resourceInstanceListOptions.ResourceID = &resourceId
	}

	next_url := ""
	var instances []rc.ResourceInstance
	for {
		if next_url != "" {
			resourceInstanceListOptions.Start = &next_url
		}
		listInstanceResponse, resp, err := rsConClient.ListResourceInstances(&resourceInstanceListOptions)
		if err != nil {
			return diag.FromErr(fmt.Errorf("[ERROR] Error retrieving resource instance: %s with resp code: %s", err, resp))
		}
		next_url, err = getInstancesNext(listInstanceResponse.NextURL)
		if err != nil {
			return diag.FromErr(fmt.Errorf("[DEBUG] ListResourceInstances failed. Error occurred while parsing NextURL: %s", err))
		}
		instances = append(instances, listInstanceResponse.Resources...)
		if next_url == "" {
			break
		}
	}

	var filteredInstances []rc.ResourceInstance
	var location string

	if loc, ok := d.GetOk("location"); ok {
		location = loc.(string)
		for _, instance := range instances {
			if flex.GetLocationV2(instance) == location {
				filteredInstances = append(filteredInstances, instance)
			}
		}
	} else {
		filteredInstances = instances
	}

	if len(filteredInstances) == 0 {
		return diag.FromErr(fmt.Errorf("[ERROR] No resource instance found with name [%s]\nIf not specified please specify more filters like resource_group_id if instance doesn't exists in default group, location or service", name))
	}
	var instance rc.ResourceInstance
	if len(filteredInstances) > 1 {
		return diag.FromErr(fmt.Errorf(
			"[ERROR] More than one resource instance found with name matching [%s]\nIf not specified please specify more filters like resource_group_id if instance doesn't exists in default group, location or service", name))
	}
	instance = filteredInstances[0]

	d.SetId(*instance.ID)
	d.Set("status", instance.State)
	d.Set("resource_group_id", instance.ResourceGroupID)
	d.Set("location", instance.RegionID)

	serviceOff, err := rsCatRepo.GetServiceName(*instance.ResourceID)
	if err != nil {
		return diag.FromErr(fmt.Errorf("[ERROR] Error retrieving service offering: %s", err))
	}

	d.Set("service", serviceOff)
	d.Set("guid", instance.GUID)
	if len(instance.Extensions) == 0 {
		d.Set("extensions", instance.Extensions)
	} else {
		d.Set("extensions", flex.Flatten(instance.Extensions))
	}
	if instance.Parameters != nil {
		instanceParameters := flex.Flatten(instance.Parameters)

		if endpoint, ok := instanceParameters["allowed_network"]; ok {
			if endpoint != "private-only" {
				endpoint = "public-and-private"
			}
			d.Set("service_endpoints", endpoint)
		} else {
			d.Set("service_endpoints", "public-and-private")
		}
		if u, ok := instanceParameters["units"]; ok {
			units, err := strconv.Atoi(u)
			if err != nil {
				log.Println("[ERROR] Error converting units from string to integer")
			}
			d.Set("units", units)
		}
		if f, ok := instanceParameters["failover_units"]; ok {
			failover_units, err := strconv.Atoi(f)
			if err != nil {
				log.Println("[ERROR] Error failover_units units from string to integer")
			}
			d.Set("failover_units", failover_units)
		}
	}

	servicePlan, err := rsCatRepo.GetServicePlanName(*instance.ResourcePlanID)
	if err != nil {
		return diag.FromErr(fmt.Errorf("[ERROR] Error retrieving plan: %s", err))
	}
	d.Set("plan", servicePlan)
	d.Set("crn", instance.CRN)

	ci, err := hsmClient(d, meta)
	if err != nil {
		return diag.FromErr(err)
	}
	ci.InstanceId = *instance.GUID
	hsmInfo, err := tkesdk.Query(ci)
	if err != nil {
		return diag.FromErr(fmt.Errorf("[ERROR] Error Quering HSM config %s", err))
	}
	d.Set("hsm_info", FlattenHSMInfo(hsmInfo))

	return nil
}

func FlattenHSMInfo(hsmInfo []tkesdk.HsmInfo) []map[string]interface{} {
	info := make([]map[string]interface{}, 0)
	for _, h := range hsmInfo {
		hsm := make(map[string]interface{})
		hsm["hsm_id"] = h.HsmId
		hsm["hsm_location"] = h.HsmLocation
		hsm["hsm_type"] = h.HsmType
		hsm["signature_threshold"] = h.SignatureThreshold
		hsm["revocation_threshold"] = h.RevocationThreshold
		hsm["current_mk_status"] = h.CurrentMKStatus
		hsm["new_mk_status"] = h.NewMKStatus
		hsm["current_mkvp"] = h.CurrentMKVP
		hsm["new_mkvp"] = h.NewMKVP
		admin := make([]map[string]interface{}, 0)
		for _, a := range h.Admins {
			ad := make(map[string]interface{})
			ad["name"] = a.AdminName
			ad["ski"] = a.AdminSKI
			admin = append(admin, ad)
		}
		hsm["admins"] = admin
		info = append(info, hsm)
	}
	return info
}

func getInstancesNext(next *string) (string, error) {
	if reflect.ValueOf(next).IsNil() {
		return "", nil
	}
	u, err := url.Parse(*next)
	if err != nil {
		return "", err
	}
	q := u.Query()
	return q.Get("next_url"), nil
}
