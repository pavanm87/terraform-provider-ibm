// Copyright IBM Corp. 2017, 2021 All Rights Reserved.
// Licensed under the Mozilla Public License v2.0

package power

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/IBM-Cloud/power-go-client/clients/instance"
	"github.com/IBM-Cloud/power-go-client/power/client/p_cloud_networks"
	"github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/IBM-Cloud/terraform-provider-ibm/ibm/conns"
	"github.com/IBM-Cloud/terraform-provider-ibm/ibm/flex"
	"github.com/IBM-Cloud/terraform-provider-ibm/ibm/validate"
	"github.com/apparentlymart/go-cidr/cidr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/customdiff"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func ResourceIBMPINetwork() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceIBMPINetworkCreate,
		ReadContext:   resourceIBMPINetworkRead,
		UpdateContext: resourceIBMPINetworkUpdate,
		DeleteContext: resourceIBMPINetworkDelete,
		Importer:      &schema.ResourceImporter{},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(15 * time.Minute),
			Update: schema.DefaultTimeout(10 * time.Minute),
			Delete: schema.DefaultTimeout(10 * time.Minute),
		},
		CustomizeDiff: customdiff.Sequence(
			func(_ context.Context, diff *schema.ResourceDiff, v interface{}) error {
				return flex.ResourcePowerUserTagsCustomizeDiff(diff)
			},
		),

		Schema: map[string]*schema.Schema{
			// Arguments
			Arg_Advertise: {
				Description:  "Enable the network to be advertised.",
				Default:      Enable,
				Optional:     true,
				Type:         schema.TypeString,
				ValidateFunc: validation.StringInSlice([]string{Enable, Disable}, false),
			},
			Arg_ARPBroadcast: {
				Description:  "Enable ARP Broadcast.",
				Default:      Disable,
				Optional:     true,
				Type:         schema.TypeString,
				ValidateFunc: validation.StringInSlice([]string{Enable, Disable}, false),
			},
			Arg_Cidr: {
				Computed:    true,
				Description: "The network CIDR. Required for `vlan` network type.",
				Optional:    true,
				Type:        schema.TypeString,
			},
			Arg_CloudInstanceID: {
				Description:  "The GUID of the service instance associated with an account.",
				ForceNew:     true,
				Required:     true,
				Type:         schema.TypeString,
				ValidateFunc: validation.NoZeroValues,
			},
			Arg_DNS: {
				Computed:    true,
				Description: "The DNS Servers for the network.",
				Elem:        &schema.Schema{Type: schema.TypeString},
				Optional:    true,
				Type:        schema.TypeSet,
			},
			Arg_Gateway: {
				Computed:    true,
				Description: "The gateway ip address.",
				Optional:    true,
				Type:        schema.TypeString,
			},
			Arg_IPAddressRange: {
				Computed:    true,
				Description: "List of one or more ip address range(s).",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						Arg_EndingIPAddress: {
							Description:  "The ending ip address.",
							Required:     true,
							Type:         schema.TypeString,
							ValidateFunc: validation.NoZeroValues,
						},
						Arg_StartingIPAddress: {
							Description:  "The staring ip address.",
							Required:     true,
							Type:         schema.TypeString,
							ValidateFunc: validation.NoZeroValues,
						},
					},
				},
				Optional: true,
				Type:     schema.TypeList,
			},
			Arg_NetworkMTU: {
				Computed:    true,
				Description: "Maximum Transmission Unit option of the network. Minimum is 1450 and maximum is 9000.",
				Optional:    true,
				Type:        schema.TypeInt,
			},
			Arg_NetworkName: {
				Description:  "The name of the network.",
				Required:     true,
				Type:         schema.TypeString,
				ValidateFunc: validation.NoZeroValues,
			},
			Arg_NetworkPeer: {
				Deprecated:  "This field is deprecated",
				Description: "Network peer information.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						Attr_ID: {
							Deprecated:  "This field is deprecated",
							Description: "ID of the network peer.",
							Required:    true,
							Type:        schema.TypeString,
						},
						Attr_NetworkAddressTranslation: {
							Deprecated:  "This field is deprecated",
							Description: "Contains the network address translation Details.",
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									Attr_SourceIP: {
										Deprecated:  "This field is deprecated",
										Description: "source IP address, required if network peer type is L3BGP or L3STATIC and if NAT is enabled.",
										Required:    true,
										Type:        schema.TypeString,
									},
								},
							},
							MaxItems: 1,
							Optional: true,
							Type:     schema.TypeList,
						},
						Attr_Type: {
							Deprecated:   "This field is deprecated",
							Description:  "Type of the network peer.",
							Optional:     true,
							Type:         schema.TypeString,
							ValidateFunc: validate.ValidateAllowedStringValues([]string{L2, L3BGP, L3Static}),
						},
					},
				},
				ForceNew: true,
				MaxItems: 1,
				Optional: true,
				Type:     schema.TypeList,
			},
			Arg_NetworkType: {
				Description:  "The type of network that you want to create. Valid values are `pub-vlan`, and `vlan`.",
				Required:     true,
				Type:         schema.TypeString,
				ValidateFunc: validate.ValidateAllowedStringValues([]string{PubVlan, Vlan}),
			},
			Arg_UserTags: {
				Computed:    true,
				Description: "The user tags attached to this resource.",
				Elem:        &schema.Schema{Type: schema.TypeString},
				Optional:    true,
				Set:         schema.HashString,
				Type:        schema.TypeSet,
			},

			// Attributes
			Attr_CRN: {
				Computed:    true,
				Description: "The CRN of this resource.",
				Type:        schema.TypeString,
			},
			Attr_NetworkAddressTranslation: {
				Computed:    true,
				Deprecated:  "This field is deprecated",
				Description: "Contains the Network Address Translation Details (for on-prem locations only).",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						Attr_SourceIP: {
							Computed:    true,
							Deprecated:  "This field is deprecated",
							Description: "source IP address, required if network peer type is L3BGP or L3STATIC and if NAT is enabled.",
							Type:        schema.TypeString,
						},
					},
				},
				Type: schema.TypeList,
			},
			Attr_NetworkID: {
				Computed:    true,
				Description: "The unique identifier of the network.",
				Type:        schema.TypeString,
			},
			Attr_PeerID: {
				Computed:    true,
				Deprecated:  "This field is deprecated",
				Description: "Network Peer ID (for on-prem locations only).",
				Type:        schema.TypeString,
			},
			Attr_VLanID: {
				Computed:    true,
				Description: "The ID of the VLAN that your network is attached to.",
				Type:        schema.TypeFloat,
			},
		},
	}
}

func resourceIBMPINetworkCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	sess, err := meta.(conns.ClientSession).IBMPISession()
	if err != nil {
		return diag.FromErr(err)
	}
	cloudInstanceID := d.Get(Arg_CloudInstanceID).(string)
	networkname := d.Get(Arg_NetworkName).(string)
	networktype := d.Get(Arg_NetworkType).(string)

	client := instance.NewIBMPINetworkClient(ctx, sess, cloudInstanceID)
	var body = &models.NetworkCreate{
		Type: &networktype,
		Name: networkname,
	}
	if v, ok := d.GetOk(Arg_DNS); ok {
		networkdns := flex.ExpandStringList((v.(*schema.Set)).List())
		if len(networkdns) > 0 {
			body.DNSServers = networkdns
		}
	}
	if tags, ok := d.GetOk(Arg_UserTags); ok {
		body.UserTags = flex.FlattenSet(tags.(*schema.Set))
	}
	if v, ok := d.GetOk(Arg_NetworkMTU); ok {
		var mtu int64 = int64(v.(int))
		body.Mtu = &mtu
	}
	if _, ok := d.GetOk(Arg_NetworkPeer); ok {
		peerModel := networkMapToNetworkCreatePeer(d.Get(Arg_NetworkPeer + ".0").(map[string]interface{}))
		body.Peer = peerModel
	}

	if networktype == Vlan {
		var networkcidr string
		var ipBodyRanges []*models.IPAddressRange
		if v, ok := d.GetOk(Arg_Cidr); ok {
			networkcidr = v.(string)
		} else {
			return diag.Errorf("%s is required when %s is vlan", Arg_Cidr, Arg_NetworkType)
		}

		gateway, firstip, lastip, err := generateIPData(networkcidr)
		if err != nil {
			return diag.FromErr(err)
		}

		ipBodyRanges = []*models.IPAddressRange{{EndingIPAddress: &lastip, StartingIPAddress: &firstip}}

		if g, ok := d.GetOk(Arg_Gateway); ok {
			gateway = g.(string)
		}

		if ips, ok := d.GetOk(Arg_IPAddressRange); ok {
			ipBodyRanges = getIPAddressRanges(ips.([]interface{}))
		}

		body.IPAddressRanges = ipBodyRanges
		body.Gateway = gateway
		body.Cidr = networkcidr
	}

	if _, ok := d.GetOk(Arg_Cidr); ok && networktype == PubVlan {
		return diag.Errorf("%s cannot be set when %s is pub-vlan", Arg_Cidr, Arg_NetworkType)
	}

	if !sess.IsOnPrem() {
		wsclient := instance.NewIBMPIWorkspacesClient(ctx, sess, cloudInstanceID)
		wsData, err := wsclient.Get(cloudInstanceID)
		if err != nil {
			return diag.FromErr(err)
		}
		if wsData.Capabilities[PER] {
			_, err = waitForPERWorkspaceActive(ctx, wsclient, cloudInstanceID, d.Timeout(schema.TimeoutRead))
			if err != nil {
				return diag.FromErr(err)
			}

			if networktype == Vlan {
				if v, ok := d.GetOk(Arg_Advertise); ok {
					body.Advertise = flex.PtrToString(v.(string))
				}
				if v, ok := d.GetOk(Arg_ARPBroadcast); ok {
					body.ArpBroadcast = flex.PtrToString(v.(string))
				}
			}
		}
	}

	networkResponse, err := createNetworkWithRetry(ctx, client, body)
	if err != nil {
		return diag.FromErr(err)
	}

	networkID := *networkResponse.NetworkID

	d.SetId(fmt.Sprintf("%s/%s", cloudInstanceID, networkID))

	_, err = isWaitForIBMPINetworkAvailable(ctx, client, networkID, d.Timeout(schema.TimeoutCreate))
	if err != nil {
		return diag.FromErr(err)
	}

	if _, ok := d.GetOk(Arg_UserTags); ok {
		if networkResponse.Crn != "" {
			oldList, newList := d.GetChange(Arg_UserTags)
			err := flex.UpdateGlobalTagsUsingCRN(oldList, newList, meta, string(networkResponse.Crn), "", UserTagType)
			if err != nil {
				log.Printf("Error on update of pi snapshot (%s) pi_user_tags during creation: %s", networkID, err)
			}
		}
	}

	return resourceIBMPINetworkRead(ctx, d, meta)
}

func resourceIBMPINetworkRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	sess, err := meta.(conns.ClientSession).IBMPISession()
	if err != nil {
		return diag.FromErr(err)
	}

	cloudInstanceID, networkID, err := splitID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	networkC := instance.NewIBMPINetworkClient(ctx, sess, cloudInstanceID)
	networkdata, err := networkC.Get(networkID)
	if err != nil {
		return diag.FromErr(err)
	}
	if networkdata.Crn != "" {
		d.Set(Attr_CRN, networkdata.Crn)
		tags, err := flex.GetGlobalTagsUsingCRN(meta, string(networkdata.Crn), "", UserTagType)
		if err != nil {
			log.Printf("Error on get of pi network (%s) pi_user_tags: %s", *networkdata.NetworkID, err)
		}
		d.Set(Arg_UserTags, tags)
	}

	if !sess.IsOnPrem() {
		wsclient := instance.NewIBMPIWorkspacesClient(ctx, sess, cloudInstanceID)
		wsData, err := wsclient.Get(cloudInstanceID)
		if err != nil {
			return diag.FromErr(err)
		}
		if wsData.Capabilities[PER] {
			_, err = waitForPERWorkspaceActive(ctx, wsclient, cloudInstanceID, d.Timeout(schema.TimeoutRead))
			if err != nil {
				return diag.FromErr(err)
			}

			if *networkdata.Type == Vlan {
				d.Set(Arg_Advertise, networkdata.Advertise)
				d.Set(Arg_ARPBroadcast, networkdata.ArpBroadcast)
			}
		}
	}

	d.Set(Arg_Cidr, networkdata.Cidr)
	d.Set(Arg_DNS, networkdata.DNSServers)
	d.Set(Arg_Gateway, networkdata.Gateway)
	d.Set(Arg_NetworkMTU, networkdata.Mtu)
	d.Set(Arg_NetworkName, networkdata.Name)
	d.Set(Arg_NetworkType, networkdata.Type)
	d.Set(Attr_NetworkID, networkdata.NetworkID)
	networkAddressTranslation := []map[string]interface{}{}
	if networkdata.NetworkAddressTranslation != nil {
		natMap := networkAddressTranslationToMap(networkdata.NetworkAddressTranslation)
		networkAddressTranslation = append(networkAddressTranslation, natMap)
	}
	d.Set(Attr_NetworkAddressTranslation, networkAddressTranslation)
	d.Set(Attr_PeerID, networkdata.PeerID)
	d.Set(Attr_VLanID, networkdata.VlanID)
	ipRangesMap := []map[string]interface{}{}
	if networkdata.IPAddressRanges != nil {
		for _, n := range networkdata.IPAddressRanges {
			if n != nil {
				v := map[string]interface{}{
					Arg_EndingIPAddress:   n.EndingIPAddress,
					Arg_StartingIPAddress: n.StartingIPAddress,
				}
				ipRangesMap = append(ipRangesMap, v)
			}
		}
	}
	d.Set(Arg_IPAddressRange, ipRangesMap)

	return nil
}

func resourceIBMPINetworkUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	sess, err := meta.(conns.ClientSession).IBMPISession()
	if err != nil {
		return diag.FromErr(err)
	}

	cloudInstanceID, networkID, err := splitID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	if d.HasChanges(Arg_Advertise, Arg_ARPBroadcast, Arg_DNS, Arg_Gateway, Arg_IPAddressRange, Arg_NetworkName) {
		client := instance.NewIBMPINetworkClient(ctx, sess, cloudInstanceID)
		body := &models.NetworkUpdate{}

		if d.HasChange(Arg_Advertise) {
			body.Advertise = d.Get(Arg_Advertise).(string)
		}

		if d.HasChange(Arg_ARPBroadcast) {
			body.ArpBroadcast = d.Get(Arg_ARPBroadcast).(string)
		}

		if d.HasChange(Arg_DNS) {
			body.DNSServers = flex.ExpandStringList((d.Get(Arg_DNS).(*schema.Set)).List())
		}

		networkType := d.Get(Arg_NetworkType).(string)
		if d.HasChange(Arg_IPAddressRange) || d.HasChange(Arg_Gateway) {
			if networkType == Vlan {
				if d.HasChange(Arg_IPAddressRange) {
					body.IPAddressRanges = getIPAddressRanges(d.Get(Arg_IPAddressRange).([]interface{}))
				}
				if d.HasChange(Arg_Gateway) {
					body.Gateway = flex.PtrToString(d.Get(Arg_Gateway).(string))
				}
			} else {
				return diag.Errorf("%v type does not allow ip-address range or gateway update", networkType)
			}
		}

		if d.HasChange(Arg_NetworkName) {
			body.Name = flex.PtrToString(d.Get(Arg_NetworkName).(string))
		}

		_, err = client.Update(networkID, body)
		if err != nil {
			return diag.FromErr(err)
		}

		_, err = isWaitForIBMPINetworkUpdated(ctx, client, *body, networkID, d.Timeout(schema.TimeoutUpdate))
		if err != nil {
			return diag.FromErr(err)
		}
	}

	if d.HasChange(Arg_UserTags) {
		if crn, ok := d.GetOk(Attr_CRN); ok {
			oldList, newList := d.GetChange(Arg_UserTags)
			err := flex.UpdateGlobalTagsUsingCRN(oldList, newList, meta, crn.(string), "", UserTagType)
			if err != nil {
				log.Printf("Error on update of pi network (%s) pi_user_tags: %s", networkID, err)
			}
		}
	}

	return resourceIBMPINetworkRead(ctx, d, meta)
}

func resourceIBMPINetworkDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	log.Printf("Calling the network delete functions. ")
	sess, err := meta.(conns.ClientSession).IBMPISession()
	if err != nil {
		return diag.FromErr(err)
	}

	cloudInstanceID, networkID, err := splitID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	client := instance.NewIBMPINetworkClient(ctx, sess, cloudInstanceID)
	err = deleteNetworkWithRetry(ctx, client, networkID)
	if err != nil {
		return diag.FromErr(err)
	}

	_, err = isWaitForIBMPINetworkDeleted(ctx, client, networkID, d.Timeout(schema.TimeoutDelete))
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}

func isWaitForIBMPINetworkAvailable(ctx context.Context, client *instance.IBMPINetworkClient, id string, timeout time.Duration) (interface{}, error) {
	stateConf := &retry.StateChangeConf{
		Pending:    []string{State_Retry, State_Build},
		Target:     []string{State_Available},
		Refresh:    isIBMPINetworkRefreshFunc(client, id),
		Timeout:    timeout,
		Delay:      10 * time.Second,
		MinTimeout: 10 * time.Second,
	}

	return stateConf.WaitForStateContext(ctx)
}

func isIBMPINetworkRefreshFunc(client *instance.IBMPINetworkClient, id string) retry.StateRefreshFunc {
	return func() (interface{}, string, error) {
		network, err := client.Get(id)
		if err != nil {
			return nil, "", err
		}

		if network.VlanID != nil {
			return network, State_Available, nil
		}

		return network, State_Build, nil
	}
}

func isWaitForIBMPINetworkDeleted(ctx context.Context, client *instance.IBMPINetworkClient, id string, timeout time.Duration) (interface{}, error) {
	stateConf := &retry.StateChangeConf{
		Pending:    []string{State_Found},
		Target:     []string{State_NotFound},
		Refresh:    isIBMPINetworkRefreshDeleteFunc(client, id),
		Timeout:    timeout,
		Delay:      10 * time.Second,
		MinTimeout: 10 * time.Second,
	}

	return stateConf.WaitForStateContext(ctx)
}

func isIBMPINetworkRefreshDeleteFunc(client *instance.IBMPINetworkClient, id string) retry.StateRefreshFunc {
	return func() (interface{}, string, error) {
		network, err := client.Get(id)
		if err != nil {
			return network, State_NotFound, nil
		}
		return network, State_Found, nil
	}
}

func isWaitForIBMPINetworkUpdated(ctx context.Context, client *instance.IBMPINetworkClient, updateBody models.NetworkUpdate, id string, timeout time.Duration) (interface{}, error) {
	stateConf := &retry.StateChangeConf{
		Pending:    []string{State_Retry},
		Target:     []string{State_Available},
		Refresh:    isIBMPINetworkRefreshUpdateFunc(client, updateBody, id),
		Timeout:    timeout,
		Delay:      10 * time.Second,
		MinTimeout: 10 * time.Second,
	}
	return stateConf.WaitForStateContext(ctx)
}

func isIBMPINetworkRefreshUpdateFunc(client *instance.IBMPINetworkClient, updateBody models.NetworkUpdate, id string) retry.StateRefreshFunc {
	return func() (interface{}, string, error) {
		network, err := client.Get(id)
		if err != nil {
			return nil, "", err
		}

		if updateBody.Advertise != "" {
			if updateBody.Advertise != network.Advertise {
				return network, State_Retry, nil
			}
		}

		if updateBody.ArpBroadcast != "" {
			if updateBody.ArpBroadcast != network.ArpBroadcast {
				return network, State_Retry, nil
			}
		}

		if len(updateBody.DNSServers) > 0 {
			sort.Strings(updateBody.DNSServers)
			sort.Strings(network.DNSServers)

			if len(updateBody.DNSServers) != len(network.DNSServers) {
				return network, State_Retry, nil
			}

			for index, dnsServer := range network.DNSServers {
				if dnsServer != network.DNSServers[index] {
					return network, State_Retry, nil
				}
			}
		}

		if updateBody.Gateway != nil {
			if *updateBody.Gateway != network.Gateway {
				return network, State_Retry, nil
			}
		}

		/*
		 * This comparison is a little tricky. The elements in the IPAddressRanges array may not come back
		 * the same way they were set in the update body. In order to circumvent this, I'm going to grab
		 * each IPAddressRange and combine it into one string put it in a list and sort it. This should
		 * ensure a 1 to 1 comparison even if it is a little more work on the terraform side.
		 */
		if len(updateBody.IPAddressRanges) > 0 {
			if len(updateBody.IPAddressRanges) != len(network.IPAddressRanges) {
				return network, State_Retry, nil
			}

			updateBodyIPAddressRanges := make([]string, 0, len(updateBody.IPAddressRanges))
			networkIPAddressRanges := make([]string, 0, len(updateBody.IPAddressRanges))

			for index := range len(updateBody.IPAddressRanges) {
				updateBodyIPAddressRanges = append(updateBodyIPAddressRanges,
					*updateBody.IPAddressRanges[index].StartingIPAddress+"-"+*updateBody.IPAddressRanges[index].EndingIPAddress)
				networkIPAddressRanges = append(networkIPAddressRanges,
					*network.IPAddressRanges[index].StartingIPAddress+"-"+*network.IPAddressRanges[index].EndingIPAddress)
			}

			sort.Strings(updateBodyIPAddressRanges)
			sort.Strings(networkIPAddressRanges)
			for index := range len(updateBody.IPAddressRanges) {
				if updateBodyIPAddressRanges[index] != networkIPAddressRanges[index] {
					return network, State_Retry, nil
				}
			}
		}

		if updateBody.Name != nil {
			if *updateBody.Name != *network.Name {
				return network, State_Retry, nil
			}
		}

		return network, State_Available, nil
	}
}

func generateIPData(cdir string) (gway, firstip, lastip string, err error) {
	_, ipv4Net, err := net.ParseCIDR(cdir)

	if err != nil {
		return "", "", "", err
	}

	var subnetToSize = map[string]int{
		"21": 2048,
		"22": 1024,
		"23": 512,
		"24": 256,
		"25": 128,
		"26": 64,
		"27": 32,
		"28": 16,
		"29": 8,
		"30": 4,
		"31": 2,
	}

	gateway, err := cidr.Host(ipv4Net, 1)
	if err != nil {
		log.Printf("Failed to get the gateway for this cidr passed in %s", cdir)
		return "", "", "", err
	}
	ad := cidr.AddressCount(ipv4Net)

	convertedad := strconv.FormatUint(ad, 10)
	// Powervc in wdc04 has to reserve 3 ip address hence we start from the 4th. This will be the default behaviour
	firstusable, err := cidr.Host(ipv4Net, 4)
	if err != nil {
		log.Print(err)
		return "", "", "", err
	}
	lastusable, err := cidr.Host(ipv4Net, subnetToSize[convertedad]-2)
	if err != nil {
		log.Print(err)
		return "", "", "", err
	}
	return gateway.String(), firstusable.String(), lastusable.String(), nil

}

func getIPAddressRanges(ipAddressRanges []interface{}) []*models.IPAddressRange {
	ipRanges := make([]*models.IPAddressRange, 0, len(ipAddressRanges))
	for _, v := range ipAddressRanges {
		if v != nil {
			ipAddressRange := v.(map[string]interface{})
			ipRange := &models.IPAddressRange{
				EndingIPAddress:   flex.PtrToString(ipAddressRange[Arg_EndingIPAddress].(string)),
				StartingIPAddress: flex.PtrToString(ipAddressRange[Arg_StartingIPAddress].(string)),
			}
			ipRanges = append(ipRanges, ipRange)
		}
	}
	return ipRanges
}

func waitForPERWorkspaceActive(ctx context.Context, client *instance.IBMPIWorkspacesClient, id string, timeout time.Duration) (interface{}, error) {
	stateConf := &retry.StateChangeConf{
		Pending:    []string{State_Inactive, State_Configuring},
		Target:     []string{State_Active},
		Refresh:    isPERWorkspaceRefreshFunc(client, id),
		Timeout:    timeout,
		Delay:      10 * time.Second,
		MinTimeout: 10 * time.Second,
	}

	return stateConf.WaitForStateContext(ctx)
}

func isPERWorkspaceRefreshFunc(client *instance.IBMPIWorkspacesClient, id string) retry.StateRefreshFunc {
	return func() (interface{}, string, error) {
		ws, err := client.Get(id)
		if err != nil {
			return nil, "", err
		}
		// Check for backward compatibility for legacy workspace.
		if ws.Details.PowerEdgeRouter == nil {
			return ws, State_Active, nil
		}
		if *(ws.Details.PowerEdgeRouter.State) == State_Active {
			return ws, State_Active, nil
		}
		if *(ws.Details.PowerEdgeRouter.State) == State_Inactive {
			return ws, State_Inactive, nil
		}
		if *(ws.Details.PowerEdgeRouter.State) == State_Error {
			return ws, *ws.Details.PowerEdgeRouter.State, fmt.Errorf("[ERROR] workspace PER configuration failed to initialize. Please try again later")
		}

		return ws, State_Configuring, nil
	}
}

func networkMapToNetworkCreatePeer(networkCreatePeerMap map[string]interface{}) *models.NetworkCreatePeer {
	ncp := &models.NetworkCreatePeer{}
	if networkCreatePeerMap[Attr_ID].(string) != "" {
		id := networkCreatePeerMap[Attr_ID].(string)
		ncp.ID = &id
	}
	if networkCreatePeerMap[Attr_NetworkAddressTranslation] != nil && len(networkCreatePeerMap[Attr_NetworkAddressTranslation].([]interface{})) > 0 {
		networkAddressTranslationModel := natMapToNetworkAddressTranslation(networkCreatePeerMap[Attr_NetworkAddressTranslation].([]interface{})[0].(map[string]interface{}))
		ncp.NetworkAddressTranslation = networkAddressTranslationModel
	}
	if networkCreatePeerMap[Attr_Type].(string) != "" {
		ncp.Type = models.NetworkPeerType(networkCreatePeerMap[Attr_Type].(string))
	}
	return ncp
}

func natMapToNetworkAddressTranslation(networkAddressTranslationMap map[string]interface{}) *models.NetworkAddressTranslation {
	nat := &models.NetworkAddressTranslation{}
	if networkAddressTranslationMap[Attr_SourceIP].(string) != "" {
		nat.SourceIP = networkAddressTranslationMap[Attr_SourceIP].(string)
	}
	return nat
}

func networkAddressTranslationToMap(nat *models.NetworkAddressTranslation) map[string]interface{} {
	natMap := make(map[string]interface{})
	if nat.SourceIP != "" {
		natMap[Attr_SourceIP] = nat.SourceIP
	}
	return natMap
}

func createNetworkWithRetry(ctx context.Context, client *instance.IBMPINetworkClient, body *models.NetworkCreate) (*models.Network, error) {
	lastErr := ""

	stateConf := &retry.StateChangeConf{
		Pending:        []string{State_Retry},
		Target:         []string{State_Active, State_Failed},
		Refresh:        retryNetworkCreationFunc(client, body, &lastErr),
		MinTimeout:     Retry_Delay,
		NotFoundChecks: Retries,
		Timeout:        10 * time.Minute,
	}

	network, err := stateConf.WaitForStateContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s", lastErr)
	}

	networkResponse := network.(*models.Network)
	return networkResponse, nil
}

func retryNetworkCreationFunc(client *instance.IBMPINetworkClient, body *models.NetworkCreate, errPointer *string) retry.StateRefreshFunc {
	return func() (interface{}, string, error) {
		network, err := client.Create(body)
		uErr := errors.Unwrap(err)

		if err != nil {
			*errPointer = err.Error()
			switch uErr.(type) {
			case *p_cloud_networks.PcloudNetworksPostBadRequest:
				log.Printf("[DEBUG] err %s on network create", err)
				return nil, State_Failed, err
			case *p_cloud_networks.PcloudNetworksPostUnprocessableEntity:
				log.Printf("[DEBUG] err %s on network create", err)
				return nil, State_Failed, err
			}

			log.Printf("[DEBUG] err %s on network create, retrying...", err)
			return nil, State_Retry, nil
		}

		return network, State_Active, nil
	}
}

func deleteNetworkWithRetry(ctx context.Context, client *instance.IBMPINetworkClient, id string) error {
	lastErr := ""

	stateConf := &retry.StateChangeConf{
		Pending:        []string{State_Retry},
		Target:         []string{State_NotFound},
		Refresh:        retryNetworkDeleteFunc(client, id, &lastErr),
		MinTimeout:     Retry_Delay,
		NotFoundChecks: Retries,
		Timeout:        10 * time.Minute,
	}

	_, err := stateConf.WaitForStateContext(ctx)
	if err != nil {
		return fmt.Errorf("%s", lastErr)
	}

	return nil
}

func retryNetworkDeleteFunc(client *instance.IBMPINetworkClient, id string, errPointer *string) retry.StateRefreshFunc {
	return func() (interface{}, string, error) {
		err := client.Delete(id)

		if err != nil && !strings.Contains(strings.ToLower(err.Error()), NotFound) {
			*errPointer = err.Error()
			log.Printf("[DEBUG] err %s on network delete, retrying...", err)
			return nil, State_Retry, nil
		}

		return "", State_NotFound, nil
	}
}
