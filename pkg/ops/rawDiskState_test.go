package ops

import (
	"os"
	"path/filepath"

	agentConstants "github.com/kairos-io/kairos/v4/agent/pkg/constants"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const testGrubCfg = "menuentry cos {}\n"

func writeTestFile(path, content string) {
	Expect(os.MkdirAll(filepath.Dir(path), 0o755)).To(Succeed())
	Expect(os.WriteFile(path, []byte(content), 0o644)).To(Succeed())
}

var _ = Describe("Raw disk state partition", Label("raw"), func() {
	var (
		rootfs string
		state  string
		r      *RawImage
	)

	BeforeEach(func() {
		rootfs = GinkgoT().TempDir()
		state = GinkgoT().TempDir()
		writeTestFile(filepath.Join(rootfs, "etc/kairos-release"), "KAIROS_ARCH=amd64\n")
		writeTestFile(filepath.Join(rootfs, agentConstants.GrubConf), testGrubCfg)
		for _, m := range agentConstants.GetGrubModules() {
			writeTestFile(filepath.Join(rootfs, "usr/lib/grub/arm64-efi", m), "arm64")
			writeTestFile(filepath.Join(rootfs, "usr/lib/grub/x86_64-efi", m), "x86_64")
		}
		writeTestFile(filepath.Join(rootfs, "usr/share/grub/unicode.pf2"), "font")
		r = NewEFIRawImage(RawImageParams{Source: rootfs, BootActive: true})
	})

	It("writes grub config, arch modules and boot assessment files", func() {
		Expect(r.writeStateGrubFiles(state)).To(Succeed())

		cfg, err := os.ReadFile(filepath.Join(state, stateGrubDir, stateGrubCfgFile))
		Expect(err).ToNot(HaveOccurred())
		Expect(string(cfg)).To(Equal(testGrubCfg))

		for _, m := range agentConstants.GetGrubModules() {
			content, err := os.ReadFile(filepath.Join(state, stateGrubDir, "x86_64-efi", m))
			Expect(err).ToNot(HaveOccurred())
			Expect(string(content)).To(Equal("x86_64"))
		}
		Expect(filepath.Join(state, stateGrubDir, "x86_64-efi", grubFontsDir, "unicode.pf2")).To(BeARegularFile())
		Expect(filepath.Join(state, stateGrubDir, "arm64-efi")).ToNot(BeADirectory())

		Expect(filepath.Join(state, stateGrubCustomFile)).To(BeARegularFile())
		Expect(filepath.Join(state, stateBootAssessmentFile)).To(BeARegularFile())
		Expect(filepath.Join(state, stateGrubMenuFile)).ToNot(BeAnExistingFile())
	})

	It("copies grub branding when the rootfs ships it", func() {
		writeTestFile(filepath.Join(rootfs, rootfsBrandingGrubMenu), "branding")
		Expect(r.writeStateGrubFiles(state)).To(Succeed())
		content, err := os.ReadFile(filepath.Join(state, stateGrubMenuFile))
		Expect(err).ToNot(HaveOccurred())
		Expect(string(content)).To(Equal("branding"))
	})

	It("fails when a grub module for the rootfs arch is missing", func() {
		Expect(os.RemoveAll(filepath.Join(rootfs, "usr/lib/grub/x86_64-efi"))).To(Succeed())
		err := r.writeStateGrubFiles(state)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("x86_64"))
	})

	It("requires the recovery image to be built first", func() {
		_, err := r.createStatePartitionImage()
		Expect(err).To(HaveOccurred())
	})

	DescribeTable("sizes the state partition",
		func(systemImage uint, configured int64, expected uint) {
			Expect(statePartitionSize(systemImage, configured)).To(Equal(expected))
		},
		Entry("derived from the system image", uint(1000), int64(0), uint(3100)),
		Entry("configured size wins", uint(1000), int64(5000), uint(5000)),
	)
})
