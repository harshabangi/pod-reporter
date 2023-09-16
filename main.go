package main

import (
	"context"
	"fmt"
	"github.com/carlmjohnson/requests"
	"github.com/labstack/echo/v4"
	corev1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"net/http"
	"strings"
	"time"
)

type statusContent struct {
	StageName         string `json:"stage_name"`
	StartTime         string `json:"start_time"`
	Duration          string `json:"duration"`
	IsCompleted       bool   `json:"is_completed"`
	InputRecordCount  int64  `json:"input_record_count"`
	OutputRecordCount int64  `json:"output_record_count"`
	TotalTasks        int    `json:"total_tasks"`
	CompletedTasks    int    `json:"completed_tasks"`
	InProgressTasks   int    `json:"in_progress_tasks"`
	ETA               string `json:"eta"`
}

func getStatusContent(c echo.Context) error {
	namespace := c.Param("namespace")
	podName := c.Param("pod_name")
	acceptHeader := c.Request().Header.Get("Accept")

	cl, err := NewInClusterKubernetesClient()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create Kubernetes client: "+err.Error())
	}

	pod, err := cl.CoreV1().Pods(namespace).Get(context.TODO(), podName, meta.GetOptions{})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get pod information: "+err.Error())
	}

	if pod.Status.Phase != corev1.PodRunning {
		return echo.NewHTTPError(http.StatusPreconditionFailed, "Pod is not in the 'Running' state")
	}

	url := fmt.Sprintf("http://%s:8081/status", pod.Status.PodIP)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	acceptHeader, err = deriveAcceptedHeader(acceptHeader)
	if err != nil {
		return err
	}

	switch acceptHeader {
	case "application/json":
		return fetchAndRespondJSON(c, url, ctx)
	case "text/html":
		return fetchAndRespondHTML(c, url, ctx)
	default:
		return echo.NewHTTPError(http.StatusBadRequest, "Unsupported Accept header: "+acceptHeader)
	}
}

func deriveAcceptedHeader(acceptHeader string) (string, error) {
	if acceptHeader == "" {
		acceptHeader = "text/html"
	}

	supportedMediaTypes := []string{"text/html", "application/json"}
	supported := false
	for _, mt := range supportedMediaTypes {
		if strings.Contains(acceptHeader, mt) {
			acceptHeader = mt
			supported = true
			break
		}
	}
	if !supported {
		return "", echo.NewHTTPError(http.StatusNotAcceptable, "Unsupported Accept header: "+acceptHeader)
	}
	return acceptHeader, nil
}

func fetchAndRespondJSON(c echo.Context, url string, ctx context.Context) error {
	var response []statusContent
	err := requests.
		URL(url).
		Accept("application/json").
		ToJSON(&response).
		Fetch(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch status information: "+err.Error())
	}
	c.Response().Header().Set("Content-Type", "application/json")
	return c.JSON(http.StatusOK, response)
}

func fetchAndRespondHTML(c echo.Context, url string, ctx context.Context) error {
	var htmlContent string
	err := requests.
		URL(url).
		Accept("text/html").
		ToString(&htmlContent).
		Fetch(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch status information: "+err.Error())
	}
	c.Response().Header().Set("Content-Type", "text/html")
	return c.HTML(http.StatusOK, htmlContent)
}

func NewInClusterKubernetesClient() (*kubernetes.Clientset, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(cfg)
}

func main() {
	e := echo.New()
	e.GET("/v1/namespaces/:namespace/pods/:pod_name/status", getStatusContent)
	e.Logger.Fatal(e.Start(":8080"))
}
