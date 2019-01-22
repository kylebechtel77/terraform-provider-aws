package aws

import (
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/glue"

	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/helper/schema"
)

func resourceAwsGlueDevEndpoint() *schema.Resource {
	return &schema.Resource{
		Create: resourceAwsGlueDevEndpointCreate,
		Read:   resourceAwsGlueDevEndpointRead,
		Update: resourceAwsDevEndpointUpdate,
		Delete: resourceAwsDevEndpointDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},

			"extra_jars_s3_path": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"extra_python_libs_s3_path": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"number_of_nodes": {
				Type:     schema.TypeInt,
				Optional: true,
				ForceNew: true,
				Default:  5,
			},

			"public_key": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"public_keys": {
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},

			"role_arn": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"security_configuration": {
				Type: schema.TypeString,
				// TODO MinItems: 1,
				Optional: true,
				ForceNew: true,
			},

			"security_group_ids": {
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},

			"subnet_id": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
		},
	}
}

func resourceAwsGlueDevEndpointCreate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).glueconn

	var name string
	if v, ok := d.GetOk("name"); ok {
		name = v.(string)
	} else {
		name = resource.UniqueId()
	}

	createOpts := &glue.CreateDevEndpointInput{
		EndpointName: aws.String(name),
		RoleArn:      aws.String(d.Get("role_arn").(string)),
	}

	if v, ok := d.GetOk("extra_jars_s3_path"); ok {
		createOpts.SetExtraJarsS3Path(v.(string))
	}

	if v, ok := d.GetOk("extra_python_libs_s3_path"); ok {
		createOpts.SetExtraPythonLibsS3Path(v.(string))
	}

	if v, ok := d.GetOk("number_of_nodes"); ok {
		createOpts.SetNumberOfNodes(int64(v.(int)))
	}

	if v, ok := d.GetOk("public_key"); ok {
		createOpts.SetPublicKey(v.(string))
	}

	if v, ok := d.GetOk("public_keys"); ok {
		publicKeys := expandStringList(v.([]interface{}))
		createOpts.SetPublicKeys(publicKeys)
	}

	if v, ok := d.GetOk("security_configuration"); ok {
		createOpts.SetSecurityConfiguration(v.(string))
	}

	if v, ok := d.GetOk("security_group_ids"); ok {
		securityGroupIDs := expandStringList(v.([]interface{}))
		createOpts.SetSecurityGroupIds(securityGroupIDs)
	}

	if v, ok := d.GetOk("subnet_id"); ok {
		createOpts.SetSubnetId(v.(string))
	}

	log.Printf("[DEBUG] Glue dev endpoint create config: %#v", *createOpts)
	_, err := conn.CreateDevEndpoint(createOpts)
	if err != nil {
		return fmt.Errorf("error creating Glue dev endpoint: %s", err)
	}
	d.SetId(name)
	log.Printf("[INFO] Glue dev endpoint ID: %s", d.Id())

	log.Printf("[DEBUG] Waiting for Glue dev endpoint (%s) to become available", d.Id())
	stateConf := &resource.StateChangeConf{
		Pending: []string{
			"PROVISIONING",
		},
		Target:  []string{"READY"},
		Refresh: glueDevEndpointStateRefreshFunc(conn, d.Id()),
		Timeout: 10 * time.Minute,
	}
	if _, err := stateConf.WaitForState(); err != nil {
		return fmt.Errorf("error while waiting for Glue dev endpoint (%s) to become available: %s", d.Id(), err)
	}

	return resourceAwsGlueDevEndpointRead(d, meta)
}

func resourceAwsGlueDevEndpointRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).glueconn

	request := &glue.GetDevEndpointInput{
		EndpointName: aws.String(d.Id()),
	}

	endpoint, err := conn.GetDevEndpoint(request)
	if err != nil {
		if glueErr, ok := err.(awserr.Error); ok && glueErr.Code() == glue.ErrCodeEntityNotFoundException {
			log.Printf("[INFO] unable to find Glue dev endpoint and therfore it is removed from the state: %s", d.Id())
			d.SetId("")
			return nil
		}
		return fmt.Errorf("error finding Glue dev endpoint %s: %s", d.Id(), err)
	}

	if err := d.Set("name", endpoint.DevEndpoint.EndpointName); err != nil {
		return fmt.Errorf("error setting name for Glue dev endpoint (%s): %s", d.Id(), err)
	}

	if err := d.Set("extra_jars_s3_path", endpoint.DevEndpoint.ExtraJarsS3Path); err != nil {
		return fmt.Errorf("error setting extra_jars_s3_path for Glue dev endpoint (%s): %s", d.Id(), err)
	}

	if err := d.Set("extra_python_libs_s3_path", endpoint.DevEndpoint.ExtraPythonLibsS3Path); err != nil {
		return fmt.Errorf("error setting extra_python_libs_s3_path for Glue dev endpoint (%s): %s", d.Id(), err)
	}

	if err := d.Set("number_of_nodes", endpoint.DevEndpoint.NumberOfNodes); err != nil {
		return fmt.Errorf("error setting number_of_nodes for Glue dev endpoint (%s): %s", d.Id(), err)
	}

	if err := d.Set("public_key", endpoint.DevEndpoint.PublicKey); err != nil {
		return fmt.Errorf("error setting public_key for Glue dev endpoint (%s): %s", d.Id(), err)
	}

	if err := d.Set("public_keys", endpoint.DevEndpoint.PublicKeys); err != nil {
		return fmt.Errorf("error setting public_keys for Glue dev endpoint (%s): %s", d.Id(), err)
	}

	if err := d.Set("role_arn", endpoint.DevEndpoint.RoleArn); err != nil {
		return fmt.Errorf("error setting role_arn for Glue dev endpoint (%s): %s", d.Id(), err)
	}

	if err := d.Set("security_configuration", endpoint.DevEndpoint.SecurityConfiguration); err != nil {
		return fmt.Errorf("error setting security_configuration for Glue dev endpoint (%s): %s", d.Id(), err)
	}

	if err := d.Set("security_group_ids", endpoint.DevEndpoint.SecurityGroupIds); err != nil {
		return fmt.Errorf("error setting security_group_ids for Glue dev endpoint (%s): %s", d.Id(), err)
	}

	if err := d.Set("subnet_id", endpoint.DevEndpoint.SubnetId); err != nil {
		return fmt.Errorf("error setting subnet_id for Glue dev endpoint (%s): %s", d.Id(), err)
	}
	return nil
}

func resourceAwsDevEndpointUpdate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).glueconn

	updateOpts := &glue.UpdateDevEndpointInput{
		EndpointName: aws.String(d.Get("name").(string)),
	}

	hasChanged := false

	if d.HasChange("public_keys") {
		oldRaw, newRaw := d.GetChange("public_keys")
		old := oldRaw.([]interface{})
		new := newRaw.([]interface{})
		create, remove := diffPublicKeys(expandStringList(old), expandStringList(new))
		updateOpts.SetAddPublicKeys(create)
		updateOpts.SetDeletePublicKeys(remove)

		hasChanged = true
	}

	if d.HasChange("public_key") {
		updateOpts.SetPublicKey(d.Get("public_key").(string))
		hasChanged = true
	}

	// TODO CustomLibraries
	if hasChanged {
		// TODO
		conn.UpdateDevEndpoint(updateOpts)
	}

	return resourceAwsGlueDevEndpointRead(d, meta)
}

func resourceAwsDevEndpointDelete(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).glueconn

	deleteOpts := &glue.DeleteDevEndpointInput{
		EndpointName: aws.String(d.Id()),
	}
	log.Printf("[INFO] Deleting Glue dev endpoint: %s", d.Id())

	return resource.Retry(5*time.Minute, func() *resource.RetryError {
		_, err := conn.DeleteDevEndpoint(deleteOpts)
		if err == nil {
			return nil
		}

		glueErr, ok := err.(awserr.Error)
		if !ok {
			return resource.NonRetryableError(err)
		}

		if glueErr.Code() == glue.ErrCodeEntityNotFoundException {
			return nil
		}

		return resource.NonRetryableError(fmt.Errorf("error deleting Glue dev endpoint: %s", err))
	})
}

func glueDevEndpointStateRefreshFunc(conn *glue.Glue, name string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		getDevEndpointInput := &glue.GetDevEndpointInput{
			EndpointName: aws.String(name),
		}
		endpoint, err := conn.GetDevEndpoint(getDevEndpointInput)
		if err != nil {
			if isAWSErr(err, glue.ErrCodeEntityNotFoundException, "") {
				return nil, "", nil
			}
			return nil, "", err
		}

		if endpoint == nil {
			return nil, "", nil
		}

		return endpoint, *endpoint.DevEndpoint.Status, nil
	}
}

func diffPublicKeys(oldKeys, newKeys []*string) ([]*string, []*string) {
	var create []*string
	var remove []*string

	for _, oldKey := range oldKeys {
		found := false
		for _, newKey := range newKeys {
			if oldKey == newKey {
				found = true
				break
			}
		}
		if !found {
			remove = append(remove, oldKey)
		}
	}

	for _, newKey := range newKeys {
		found := false
		for _, oldKey := range oldKeys {
			if oldKey == newKey {
				found = true
				break
			}
		}
		if !found {
			create = append(remove, newKey)
		}
	}

	return create, remove
}
