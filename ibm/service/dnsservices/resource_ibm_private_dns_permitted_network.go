// Copyright IBM Corp. 2017, 2021 All Rights Reserved.
// Licensed under the Mozilla Public License v2.0

package dnsservices

import (
	"fmt"
	"strings"
	"time"

	"github.com/IBM-Cloud/terraform-provider-ibm/ibm/conns"
	"github.com/IBM-Cloud/terraform-provider-ibm/ibm/flex"
	"github.com/IBM-Cloud/terraform-provider-ibm/ibm/validate"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const (
	pdnsVpcCRN                     = "vpc_crn"
	pdnsNetworkType                = "type"
	pdnsPermittedNetworkID         = "permitted_network_id"
	pdnsPermittedNetworkCreatedOn  = "created_on"
	pdnsPermittedNetworkModifiedOn = "modified_on"
	pdnsPermittedNetworkState      = "state"
	pdnsPermittedNetwork           = "permitted_network"
)

var allowedNetworkTypes = []string{
	"vpc",
}

func ResourceIBMPrivateDNSPermittedNetwork() *schema.Resource {
	return &schema.Resource{
		Create:   resourceIBMPrivateDNSPermittedNetworkCreate,
		Read:     resourceIBMPrivateDNSPermittedNetworkRead,
		Delete:   resourceIBMPrivateDNSPermittedNetworkDelete,
		Exists:   resourceIBMPrivateDNSPermittedNetworkExists,
		Importer: &schema.ResourceImporter{},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(10 * time.Minute),
			Delete: schema.DefaultTimeout(10 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			pdnsPermittedNetworkID: {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Network Id",
			},

			pdnsInstanceID: {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Instance Id",
			},

			pdnsZoneID: {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Zone Id",
			},

			pdnsNetworkType: {
				Type:         schema.TypeString,
				Optional:     true,
				ForceNew:     true,
				Default:      "vpc",
				ValidateFunc: validate.ValidateAllowedStringValues([]string{"vpc"}),
				Description:  "Network Type",
			},

			pdnsVpcCRN: {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "VPC CRN id",
			},

			pdnsPermittedNetworkCreatedOn: {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Network creation date",
			},

			pdnsPermittedNetworkModifiedOn: {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Network Modification date",
			},

			pdnsPermittedNetworkState: {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Network status",
			},
		},
	}
}

func resourceIBMPrivateDNSPermittedNetworkCreate(d *schema.ResourceData, meta interface{}) error {
	sess, err := meta.(conns.ClientSession).PrivateDNSClientSession()
	if err != nil {
		return err
	}

	instanceID := d.Get(pdnsInstanceID).(string)
	zoneID := d.Get(pdnsZoneID).(string)
	vpcCRN := d.Get(pdnsVpcCRN).(string)
	nwType := d.Get(pdnsNetworkType).(string)
	mk := "private_dns_permitted_network_" + instanceID + zoneID
	conns.IbmMutexKV.Lock(mk)
	defer conns.IbmMutexKV.Unlock(mk)

	permittedNetworkCrn, err := sess.NewPermittedNetworkVpc(vpcCRN)
	if err != nil {
		return err
	}
	createPermittedNetworkOptions := sess.NewCreatePermittedNetworkOptions(instanceID, zoneID, nwType, permittedNetworkCrn)

	response, detail, err := sess.CreatePermittedNetwork(createPermittedNetworkOptions)
	if err != nil {
		return flex.FmtErrorf("[ERROR] Error creating dns services permitted network:%s\n%s", err, detail)
	}

	d.SetId(fmt.Sprintf("%s/%s/%s", instanceID, zoneID, *response.ID))

	return resourceIBMPrivateDNSPermittedNetworkRead(d, meta)
}

func resourceIBMPrivateDNSPermittedNetworkRead(d *schema.ResourceData, meta interface{}) error {
	sess, err := meta.(conns.ClientSession).PrivateDNSClientSession()
	if err != nil {
		return err
	}

	idSet := strings.Split(d.Id(), "/")
	getPermittedNetworkOptions := sess.NewGetPermittedNetworkOptions(idSet[0], idSet[1], idSet[2])
	response, detail, err := sess.GetPermittedNetwork(getPermittedNetworkOptions)

	if err != nil {
		return flex.FmtErrorf("[ERROR] Error reading dns services permitted network:%s\n%s", err, detail)
	}

	d.Set(pdnsInstanceID, idSet[0])
	d.Set(pdnsZoneID, idSet[1])
	d.Set(pdnsPermittedNetworkID, response.ID)
	d.Set(pdnsPermittedNetworkCreatedOn, response.CreatedOn.String())
	d.Set(pdnsPermittedNetworkModifiedOn, response.ModifiedOn.String())
	d.Set(pdnsVpcCRN, response.PermittedNetwork.VpcCrn)
	d.Set(pdnsNetworkType, response.Type)
	d.Set(pdnsPermittedNetworkState, response.State)

	return nil
}

func resourceIBMPrivateDNSPermittedNetworkDelete(d *schema.ResourceData, meta interface{}) error {
	sess, err := meta.(conns.ClientSession).PrivateDNSClientSession()
	if err != nil {
		return err
	}

	idSet := strings.Split(d.Id(), "/")
	mk := "private_dns_permitted_network_" + idSet[0] + idSet[1]
	conns.IbmMutexKV.Lock(mk)
	defer conns.IbmMutexKV.Unlock(mk)
	deletePermittedNetworkOptions := sess.NewDeletePermittedNetworkOptions(idSet[0], idSet[1], idSet[2])
	_, response, err := sess.DeletePermittedNetwork(deletePermittedNetworkOptions)

	if err != nil {
		return flex.FmtErrorf("[ERROR] Error deleting dns services permitted network:%s\n%s", err, response)
	}

	d.SetId("")
	return nil
}

func resourceIBMPrivateDNSPermittedNetworkExists(d *schema.ResourceData, meta interface{}) (bool, error) {
	sess, err := meta.(conns.ClientSession).PrivateDNSClientSession()
	if err != nil {
		return false, err
	}

	idSet := strings.Split(d.Id(), "/")
	if len(idSet) < 3 {
		return false, flex.FmtErrorf("[ERROR] Incorrect ID %s: Id should be a combination of InstanceID/zoneID/permittedNetworkID", d.Id())
	}

	mk := "private_dns_permitted_network_" + idSet[0] + idSet[1]
	conns.IbmMutexKV.Lock(mk)
	defer conns.IbmMutexKV.Unlock(mk)
	getPermittedNetworkOptions := sess.NewGetPermittedNetworkOptions(idSet[0], idSet[1], idSet[2])
	_, response, err := sess.GetPermittedNetwork(getPermittedNetworkOptions)
	if err != nil {
		if response != nil && response.StatusCode == 404 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
