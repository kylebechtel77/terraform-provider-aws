package aws

import (
	"fmt"
	"log"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/glue"
	"github.com/hashicorp/terraform/helper/acctest"
	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/terraform"
)

const GlueDevEndpointResourcePrefix = "tf-acc-test"

func init() {
	resource.AddTestSweepers("aws_glue_dev_endpoint", &resource.Sweeper{
		Name: "aws_glue_dev_endpoint",
		F:    testSweepGlueDevEndpoint,
	})
}

func testSweepGlueDevEndpoint(region string) error {
	client, err := sharedClientForRegion(region)
	if err != nil {
		return fmt.Errorf("error getting client: %s", err)
	}
	conn := client.(*AWSClient).glueconn

	input := &glue.GetDevEndpointsInput{}
	err = conn.GetDevEndpointsPages(input, func(page *glue.GetDevEndpointsOutput, lastPage bool) bool {
		if len(page.DevEndpoints) == 0 {
			log.Printf("[INFO] No Glue Dev Endpoints to sweep")
			return false
		}
		for _, endpoint := range page.DevEndpoints {
			name := aws.StringValue(endpoint.EndpointName)
			if !strings.HasPrefix(name, GlueDevEndpointResourcePrefix) {
				log.Printf("[INFO] Skipping Glue Dev Endpoint: %s", name)
				continue
			}

			log.Printf("[INFO] Deleting Glue Dev Endpoint: %s", name)
			_, err := conn.DeleteDevEndpoint(&glue.DeleteDevEndpointInput{
				EndpointName: aws.String(name),
			})
			if err != nil {
				log.Printf("[ERROR] Failed to delete Glue Dev Endpoint %s: %s", name, err)
			}
		}
		return !lastPage
	})
	if err != nil {
		if testSweepSkipSweepError(err) {
			log.Printf("[WARN] Skipping Glue Dev Endpoint sweep for %s: %s", region, err)
			return nil
		}
		return fmt.Errorf("error retrieving Glue Dev Endpoint: %s", err)
	}

	return nil
}

func TestAccGlueDevEndpoint_Basic(t *testing.T) {
	var endpoint glue.DevEndpoint
	rName := acctest.RandomWithPrefix(GlueDevEndpointResourcePrefix)
	resourceName := "aws_glue_dev_endpoint.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSGlueDevEndpointDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccGlueDevEndpointConfig_Basic(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSGlueDevEndpointExists(resourceName, &endpoint),

					resource.TestCheckResourceAttr(resourceName, "name", rName),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCheckAWSGlueDevEndpointExists(resourceName string, endpoint *glue.DevEndpoint) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("no ID is set")
		}

		conn := testAccProvider.Meta().(*AWSClient).glueconn
		output, err := conn.GetDevEndpoint(&glue.GetDevEndpointInput{
			EndpointName: aws.String(rs.Primary.ID),
		})

		if err != nil {
			return err
		}

		if output == nil {
			return fmt.Errorf("no Glue Dev Endpoint")
		}

		*endpoint = *output.DevEndpoint

		return nil
	}
}

func testAccCheckAWSGlueDevEndpointDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "aws_glue_dev_endpoint" {
			continue
		}

		conn := testAccProvider.Meta().(*AWSClient).glueconn
		output, err := conn.GetDevEndpoint(&glue.GetDevEndpointInput{
			EndpointName: aws.String(rs.Primary.ID),
		})

		if err != nil {
			if isAWSErr(err, glue.ErrCodeEntityNotFoundException, "") {
				return nil
			}
			return err
		}

		endpoint := output.DevEndpoint
		if endpoint != nil && aws.StringValue(endpoint.EndpointName) == rs.Primary.ID {
			return fmt.Errorf("the Glue Dev Endpoint %s still exists", rs.Primary.ID)
		}

		return nil
	}

	return nil
}

func testAccGlueDevEndpointConfig_Base(rName string) string {
	return fmt.Sprintf(`
resource "aws_iam_role" "test" {
  name = "AWSGlueServiceRole-%s"
  assume_role_policy = "${data.aws_iam_policy_document.test.json}"
}

data "aws_iam_policy_document" "test" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["glue.amazonaws.com"]
    }
  }
}

resource "aws_iam_role_policy_attachment" "test-AWSGlueServiceRole" {
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSGlueServiceRole"
  role       = "${aws_iam_role.test.name}"
}
`, rName)
}

func testAccGlueDevEndpointConfig_Basic(rName string) string {
	return fmt.Sprintf(`
resource "aws_iam_role" "test" {
  name = "AWSGlueServiceRole-%s"
  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "glue.amazonaws.com"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
EOF
}

resource "aws_glue_dev_endpoint" "test" {
  name = %q
  role_arn = "arn:aws:iam::059076854262:role/bla"
}
`, rName, rName)
}
